package sync

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Altinity/docker-sync/config"
	"github.com/Altinity/docker-sync/internal/telemetry"
	"github.com/Altinity/docker-sync/structs"
	"github.com/cenkalti/backoff/v4"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func checkRateLimit(err error) error {
	if strings.Contains(err.Error(), "HAP429") || strings.Contains(err.Error(), "TOOMANYREQUESTS") {
		log.Warn().
			Msg("Rate limited by registry, backing off")
		return err
	}

	return backoff.Permanent(err)
}

func push(ctx context.Context, image *structs.Image, desc *remote.Descriptor, dst string, tag string) error {
	return backoff.RetryNotify(func() error {
		switch getRepositoryType(dst) {
		case S3CompatibleRepository:
			fields := strings.Split(dst, ":")

			var fn func(context.Context, *remote.Descriptor, string, string, string) error

			switch fields[0] {
			case "r2":
				fn = pushR2
			case "s3":
				fn = pushS3
			default:
				return fmt.Errorf("unsupported bucket destination: %s", dst)
			}

			if err := fn(ctx, desc, dst, fields[3], tag); err != nil {
				if errors.Is(err, remote.ErrSchema1) {
					return backoff.Permanent(fmt.Errorf("unsupported v1 schema"))
				}
				return err
			}

			return nil
		case OCIRepository:
			pushAuth, _ := getAuth(image.GetRegistry(dst), image.GetRepository(dst))

			pusher, err := remote.NewPusher(pushAuth)
			if err != nil {
				return err
			}

			dstTag, err := name.ParseReference(fmt.Sprintf("%s:%s", dst, tag))
			if err != nil {
				return fmt.Errorf("failed to parse tag: %w", err)
			}

			dataCounter := &ociDataCounter{
				ctx:  ctx,
				dest: dst,
				f:    desc,
			}

			if err := pusher.Push(ctx, dstTag, dataCounter); err != nil {
				// FIXME: Work around bug in go-containerregistry.
				// Unfortunately we lose the uploaded bytes telemetry.
				if strings.Contains(err.Error(), "MANIFEST_BLOB_UNKNOWN") || strings.Contains(err.Error(), "INVALID") {
					log.Warn().
						Msg("Bug in go-containerregistry, falling back to skopeo")

					skopeoSrcAuth, _ := getSkopeoAuth(image.GetSourceRegistry(), image.GetSourceRepository(), "src")
					skopeoDstAuth, _ := getSkopeoAuth(image.GetRegistry(dst), image.GetRepository(dst), "dest")

					return SkopeoCopy(ctx, fmt.Sprintf("%s:%s", image.GetSource(), tag), skopeoSrcAuth, fmt.Sprintf("%s:%s", dst, tag), skopeoDstAuth)
				}
				return checkRateLimit(err)
			}
		default:
			return fmt.Errorf("unsupported repository type")
		}

		return nil
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

func pull(ctx context.Context, puller *remote.Puller, image *structs.Image, tag string) (*remote.Descriptor, error) {
	srcTag, err := name.ParseReference(fmt.Sprintf("%s:%s", image.Source, tag))
	if err != nil {
		return nil, fmt.Errorf("failed to parse tag: %w", err)
	}

	var desc *remote.Descriptor

	if err := backoff.RetryNotify(func() error {
		desc, err = puller.Get(ctx, srcTag)
		if err != nil {
			return checkRateLimit(err)
		}
		return nil
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(
		backoff.WithInitialInterval(1*time.Minute),
	), config.SyncMaxErrors.UInt64()), func(err error, dur time.Duration) {
		log.Error().
			Err(err).
			Dur("backoff", dur).
			Str("image", image.Source).
			Str("tag", tag).
			Msg("Pull failed")
	}); err != nil {
		return nil, err
	}

	return desc, nil
}

func SyncImage(ctx context.Context, image *structs.Image) error {
	log.Info().
		Str("image", image.Source).
		Strs("targets", image.Targets).
		Msg("Syncing image")

	srcAuth, srcAuthName := getAuth(image.GetSourceRegistry(), image.GetSourceRepository())

	srcTags, err := listOCITags(ctx, srcAuth, image, image.GetSource(), "")
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

	srcPuller, err := remote.NewPuller(srcAuth)
	if err != nil {
		return err
	}

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
			auth, dstAuthName := getAuth(image.GetRegistry(dst), image.GetRepository(dst))

			tags, err := listOCITags(ctx, auth, image, dst, dst)
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

				dstTags = append(dstTags, tags...)
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

			desc, err := pull(ctx, srcPuller, image, tag)
			if err != nil {
				return err
			}

			for _, dst := range actualDsts {
				if err := push(ctx, image, desc, dst, tag); err != nil {
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
