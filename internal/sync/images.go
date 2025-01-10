package sync

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Altinity/docker-sync/config"
	"github.com/Altinity/docker-sync/internal/telemetry"
	"github.com/Altinity/docker-sync/structs"
	"github.com/cenkalti/backoff/v4"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func checkRateLimit(err error) error {
	if err == nil {
		return nil
	}

	if strings.Contains(err.Error(), "HAP429") || strings.Contains(err.Error(), "TOOMANYREQUESTS") {
		log.Warn().
			Msg("Rate limited by registry, backing off")
		return err
	}

	return backoff.Permanent(err)
}

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
			srcAuth, _ := getSkopeoAuth(image.GetSourceRegistry(), image.GetSourceRepository())
			dstAuth, _ := getSkopeoAuth(image.GetRegistry(dst), image.GetRepository(dst))

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

func SyncImage(ctx context.Context, image *structs.Image) error {
	log.Info().
		Str("image", image.Source).
		Strs("targets", image.Targets).
		Msg("Syncing image")

	srcRef, err := docker.ParseReference(fmt.Sprintf("//%s", image.Source))
	if err != nil {
		return err
	}
	image.SrcRef = srcRef

	srcAuth, srcAuthName := getSkopeoAuth(image.GetSourceRegistry(), image.GetSourceRepository())
	srcCtx := &types.SystemContext{
		DockerAuthConfig: srcAuth,
	}

	srcTags, err := docker.GetRepositoryTags(ctx, srcCtx, srcRef)
	if err != nil {
		return err
	}

	if len(srcTags) == 0 {
		log.Warn().
			Str("image", image.Source).
			Str("auth", srcAuthName).
			Msg("No source tags found, skipping image")

		return nil
	}

	log.Info().
		Str("image", image.Source).
		Str("auth", srcAuthName).
		Int("tags", len(srcTags)).
		Msg("Found source tags")

	// Get all tags from targets
	var dstTags []string

	for _, dst := range image.Targets {
		switch getRepositoryType(dst) {
		case S3CompatibleRepository:
			fields := strings.Split(dst, ":")

			tags, err := listS3Tags(dst, fields)
			if err != nil {
				return err
			}

			if len(tags) > 0 {
				log.Info().
					Str("image", image.Source).
					Str("target", dst).
					Int("tags", len(tags)).
					Msg("Found destination tags")

				dstTags = append(dstTags, tags...)
			}

			continue
		case OCIRepository:
			dstRef, err := docker.ParseReference(fmt.Sprintf("//%s", dst))
			if err != nil {
				return err
			}

			dstAuth, dstAuthName := getSkopeoAuth(image.GetRegistry(dst), image.GetRepository(dst))

			dstCtx := &types.SystemContext{
				DockerAuthConfig: dstAuth,
			}

			tags, err := docker.GetRepositoryTags(ctx, dstCtx, dstRef)
			if err != nil {
				return err
			}

			if len(tags) > 0 {
				log.Info().
					Str("image", image.Source).
					Str("target", dst).
					Int("tags", len(tags)).
					Str("auth", dstAuthName).
					Msg("Found destination tags")

				for _, tag := range tags {
					dstTags = append(dstTags, fmt.Sprintf("%s:%s", dst, tag))
				}
			}
		}
	}

	// Sync tags
	for _, tag := range srcTags {
		if slices.Contains(image.IgnoredTags, tag) {
			log.Info().
				Str("image", image.Source).
				Str("tag", tag).
				Msg("Skipping ignored tag")
			continue
		}

		telemetry.Errors.Add(ctx, 0,
			metric.WithAttributes(
				attribute.KeyValue{
					Key:   "image",
					Value: attribute.StringValue(image.Source),
				},
				attribute.KeyValue{
					Key:   "tag",
					Value: attribute.StringValue(tag),
				},
			),
		)

		if err := func() error {
			tag := tag

			var actualDsts []string

			for _, dst := range image.Targets {
				if !slices.Contains(image.MutableTags, tag) && slices.Contains(dstTags, fmt.Sprintf("%s:%s", dst, tag)) {
					continue
				}

				actualDsts = append(actualDsts, dst)
			}

			if len(actualDsts) == 0 {
				log.Info().
					Str("image", image.Source).
					Str("tag", tag).
					Msg("Tag already exists in all targets, skipping")

				return nil
			}

			log.Info().
				Str("image", image.Source).
				Str("tag", tag).
				Strs("targets", image.Targets).
				Msg("Syncing tag")

			for _, dst := range actualDsts {
				if err := push(ctx, image, dst, tag); err != nil {
					return err
				}
			}

			return nil
		}(); err != nil {
			log.Error().
				Err(err).
				Str("image", image.Source).
				Str("tag", tag).
				Msg("Failed to sync tag")

			telemetry.Errors.Add(ctx, 1,
				metric.WithAttributes(
					attribute.KeyValue{
						Key:   "image",
						Value: attribute.StringValue(image.Source),
					},
					attribute.KeyValue{
						Key:   "tag",
						Value: attribute.StringValue(tag),
					},
					attribute.KeyValue{
						Key:   "error",
						Value: attribute.StringValue(err.Error()),
					},
				),
			)
		} else {
			telemetry.Syncs.Add(ctx, 1,
				metric.WithAttributes(
					attribute.KeyValue{
						Key:   "image",
						Value: attribute.StringValue(image.Source),
					},
					attribute.KeyValue{
						Key:   "tag",
						Value: attribute.StringValue(tag),
					},
				),
			)
		}
	}

	return nil
}
