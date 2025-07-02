package sync

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Altinity/docker-sync/config"
	"github.com/Altinity/docker-sync/structs"
	"github.com/cenkalti/backoff/v4"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/rs/zerolog/log"
)

func push(ctx context.Context, image *structs.Image, dst string, tag string) error {
	return backoff.RetryNotify(func() error {
		switch getRepositoryType(dst) {
		case S3CompatibleRepository:
			fields := strings.Split(dst, ":")

			var fn func(context.Context, *structs.Image, string, string, string) error

			switch fields[0] {
			case "r2":
				fn = pushR2
			case "s3":
				fn = pushS3
			default:
				return fmt.Errorf("unsupported bucket destination: %s", dst)
			}

			if err := fn(ctx, image, dst, fields[3], tag); err != nil {
				return err
			}

			return nil
		case OCIRepository:
			srcAuth, _ := getSkopeoAuth(ctx, image.GetSourceRegistry(), image.GetSourceRepository())
			dstAuth, _ := getSkopeoAuth(ctx, image.GetRegistry(dst), image.GetRepository(dst))

			dstRef, err := docker.ParseReference(fmt.Sprintf("//%s:%s", dst, tag))
			if err != nil {
				return err
			}

			srcRef, err := docker.ParseReference(fmt.Sprintf("//%s:%s", image.Source, tag))
			if err != nil {
				return err
			}

			srcCtx := &types.SystemContext{
				DockerAuthConfig: srcAuth,
			}
			dstCtx := &types.SystemContext{
				DockerAuthConfig: dstAuth,
			}

			policy := &signature.Policy{Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()}}
			policyContext, err := signature.NewPolicyContext(policy)
			if err != nil {
				return err
			}

			ch := make(chan types.ProgressProperties)
			defer close(ch)

			chCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			go dockerDataCounter(chCtx, image.Source, dst, ch)

			_, err = copy.Image(ctx, policyContext, dstRef, srcRef, &copy.Options{
				SourceCtx:          srcCtx,
				DestinationCtx:     dstCtx,
				ImageListSelection: copy.CopyAllImages,
				ProgressInterval:   time.Second,
				Progress:           ch,
			})

			return checkRateLimit(err)
		default:
			return fmt.Errorf("unsupported repository type")
		}
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(
		backoff.WithInitialInterval(1*time.Minute),
	), config.SyncMaxErrors.UInt64()), func(err error, dur time.Duration) {
		log.Error().
			Err(err).
			Dur("backoff", dur).
			Str("image", image.Source).
			Str("tag", tag).
			Str("target", dst).
			Msg("Push failed")
	})
}
