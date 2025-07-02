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
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/cenkalti/backoff/v4"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/types"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/sync/errgroup"
)

func delete(ctx context.Context, image *structs.Image, dst string, tag string) error {
	return backoff.RetryNotify(func() error {
		switch getRepositoryType(dst) {
		case S3CompatibleRepository:
			fields := strings.Split(dst, ":")

			var fn func(context.Context, *structs.Image, string, string, string) error

			switch fields[0] {
			case "r2":
				fn = deleteR2
			case "s3":
				fn = deleteS3
			default:
				return fmt.Errorf("unsupported bucket destination: %s", dst)
			}

			if err := fn(ctx, image, dst, fields[3], tag); err != nil {
				return err
			}

			return nil
		case OCIRepository:
			dstAuth, _ := getSkopeoAuth(ctx, image.GetRegistry(dst), image.GetRepository(dst))

			dstRef, err := docker.ParseReference(fmt.Sprintf("//%s:%s", dst, tag))
			if err != nil {
				return err
			}

			dstCtx := &types.SystemContext{
				DockerAuthConfig: dstAuth,
			}

			ch := make(chan types.ProgressProperties)
			defer close(ch)

			chCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			go dockerDataCounter(chCtx, image.Source, dst, ch)

			err = dstRef.DeleteImage(chCtx, dstCtx)

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
			Msg("Delete failed")
	})
}

func purge(ctx context.Context, image *structs.Image, srcTags []string, dstTags []string) {
	if !image.Purge {
		return
	}

	// Purge tags
	for _, dst := range image.Targets {
		var toPurge []string

		for _, tag := range dstTags {
			tag = strings.TrimPrefix(tag, fmt.Sprintf("%s:", dst))
			if !slices.Contains(srcTags, tag) && !slices.Contains(image.MutableTags, tag) {
				toPurge = append(toPurge, tag)
			}
		}

		if len(toPurge) == 0 {
			log.Debug().
				Str("image", image.Source).
				Str("target", dst).
				Msg("No tags to purge in target, skipping")

			purgeOrphans(ctx, image, dst)

			continue
		}

		slices.Sort(toPurge)
		toPurge = slices.Compact(toPurge)

		log.Info().
			Str("image", image.Source).
			Str("target", dst).
			Strs("tags", toPurge).
			Msg("Purging tags")

		g, _ := errgroup.WithContext(ctx)
		g.SetLimit(config.SyncS3MaxPurgeConcurrency.Int())

		for _, tag := range toPurge {
			g.Go(func() error {
				// Initialize telemetry for the purge
				telemetry.PurgeErrors.Add(ctx, 0,
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
							Key:   "target",
							Value: attribute.StringValue(dst),
						},
					),
				)

				if err := func() error {
					return delete(ctx, image, dst, tag)
				}(); err != nil {
					log.Error().
						Err(err).
						Str("image", image.Source).
						Str("tag", tag).
						Str("target", dst).
						Msg("Failed to purge tag")

					telemetry.PurgeErrors.Add(ctx, 1,
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
								Key:   "target",
								Value: attribute.StringValue(dst),
							},
							attribute.KeyValue{
								Key:   "error",
								Value: attribute.StringValue(err.Error()),
							},
						),
					)
				}

				return nil
			})
		}

		_ = g.Wait()

		purgeOrphans(ctx, image, dst)
	}
}

func purgeOrphans(ctx context.Context, image *structs.Image, dst string) {
	// Remove orphaned blobs
	if strings.HasPrefix(dst, "r2:") || strings.HasPrefix(dst, "s3:") {
		var s3Session *s3.Client
		var bucket *string
		var err error

		if strings.HasPrefix(dst, "r2:") {
			s3Session, bucket, err = getR2Session(dst)
		} else if strings.HasPrefix(dst, "s3:") {
			s3Session, bucket, err = getS3Session(dst)
		}

		if err != nil {
			log.Error().
				Err(err).
				Str("image", image.Source).
				Str("target", dst).
				Msg("Failed to get S3 session for purge")

			telemetry.PurgeErrors.Add(ctx, 1,
				metric.WithAttributes(
					attribute.KeyValue{
						Key:   "image",
						Value: attribute.StringValue(image.Source),
					},
					attribute.KeyValue{
						Key:   "target",
						Value: attribute.StringValue(dst),
					},
					attribute.KeyValue{
						Key:   "error",
						Value: attribute.StringValue(err.Error()),
					},
				),
			)

			return
		}

		if err := deleteOrphanedBlobsS3(ctx, s3Session, *bucket, image.GetRepository(dst)); err != nil {
			log.Error().
				Err(err).
				Str("image", image.Source).
				Str("target", dst).
				Msg("Failed to delete orphaned blobs")

			telemetry.PurgeErrors.Add(ctx, 1,
				metric.WithAttributes(
					attribute.KeyValue{
						Key:   "image",
						Value: attribute.StringValue(image.Source),
					},
					attribute.KeyValue{
						Key:   "target",
						Value: attribute.StringValue(dst),
					},
					attribute.KeyValue{
						Key:   "error",
						Value: attribute.StringValue(err.Error()),
					},
				),
			)
		}
	}
}
