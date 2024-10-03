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
		pushAuth, _ := getAuth(image.GetRegistry(dst), image.GetRepository(dst))

		pusher, err := remote.NewPusher(pushAuth)
		if err != nil {
			return err
		}

		dstTag, err := name.ParseReference(fmt.Sprintf("%s:%s", dst, tag))
		if err != nil {
			return fmt.Errorf("failed to parse tag: %w", err)
		}

		if err := pusher.Push(ctx, dstTag, desc); err != nil {
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

	pullAuth, pullAuthName := getAuth(image.GetSourceRegistry(), image.GetSourceRepository())

	srcPuller, err := remote.NewPuller(pullAuth)
	if err != nil {
		return err
	}

	srcRepo, err := name.NewRepository(image.Source)
	if err != nil {
		return err
	}

	srcLister, err := srcPuller.Lister(ctx, srcRepo)
	if err != nil {
		return err
	}

	// Get all tags from source
	log.Info().
		Str("image", image.Source).
		Str("auth", pullAuthName).
		Msg("Fetching tags")

	var srcTags []string

	for srcLister.HasNext() {
		tags, err := srcLister.Next(ctx)
		if err != nil {
			return err
		}

		srcTags = append(srcTags, tags.Tags...)
	}

	log.Info().
		Str("image", image.Source).
		Str("auth", pullAuthName).
		Int("tags", len(srcTags)).
		Msg("Found tags")

	// Get all tags from targets
	var dstTags []string

	for _, dst := range image.Targets {
		dstRepo, err := name.NewRepository(dst)
		if err != nil {
			return err
		}

		pushAuth, pushAuthName := getAuth(image.GetRegistry(dst), image.GetRepository(dst))

		log.Info().
			Str("image", image.Source).
			Str("target", dst).
			Str("auth", pushAuthName).
			Msg("Fetching destination tags")

		dstPuller, err := remote.NewPuller(pushAuth)
		if err != nil {
			return err
		}

		dstLister, err := dstPuller.Lister(ctx, dstRepo)
		if err != nil {
			return err
		}

		for dstLister.HasNext() {
			tags, err := dstLister.Next(ctx)
			if err != nil {
				return err
			}

			for _, tag := range tags.Tags {
				dstTags = append(dstTags, fmt.Sprintf("%s:%s", dst, tag))
			}
		}

		log.Info().
			Str("image", image.Source).
			Str("target", dst).
			Int("tags", len(dstTags)).
			Str("auth", pushAuthName).
			Msg("Found destination tags")
	}

	// Sync tags
	for _, tag := range srcTags {
		log.Info().
			Str("image", image.Source).
			Str("tag", tag).
			Strs("targets", image.Targets).
			Msg("Syncing tag")

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

			desc, err := pull(ctx, srcPuller, image, tag)
			if err != nil {
				return err
			}

			for _, dst := range image.Targets {
				if !slices.Contains(image.MutableTags, tag) && slices.Contains(dstTags, fmt.Sprintf("%s:%s", dst, tag)) {
					log.Info().
						Str("image", image.Source).
						Str("tag", tag).
						Str("target", dst).
						Msg("Tag already exists, skipping")
					return nil
				}
				if err := push(ctx, image, desc, dst, tag); err != nil {
					return err
				}

				return nil
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
