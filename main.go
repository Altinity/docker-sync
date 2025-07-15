package dockersync

import (
	"context"
	"time"

	"github.com/Altinity/docker-sync/config"
	"github.com/Altinity/docker-sync/internal/sync"
	"github.com/Altinity/docker-sync/internal/telemetry"
	"github.com/Altinity/docker-sync/structs"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"
)

func Run(ctx context.Context) error {
	if config.TelemetryEnabled.Bool() {
		go func() {
			if err := telemetry.Start(ctx); err != nil {
				log.Error().
					Err(err).
					Msg("Failed to start telemetry")
			}
		}()
	}

	images := config.SyncImages.Images()

	for {
		if err := RunOnce(ctx, images); err != nil {
			return err
		}

		dur := config.SyncInterval.Duration()
		log.Info().Dur("interval", dur).Msg("Waiting for next sync")
		time.Sleep(dur)
	}
}

func RunOnce(ctx context.Context, images []*structs.Image) error {
	var merr error

	for k := range images {
		telemetry.MonitoredImages.Record(ctx, int64(len(images)))

		select {
		case <-ctx.Done():
			return nil
		default:
			image := images[k]

			// Initialize telemetry for the image
			telemetry.ImageSyncErrors.Add(ctx, 0,
				metric.WithAttributes(
					attribute.KeyValue{
						Key:   "image",
						Value: attribute.StringValue(image.Source),
					},
				),
			)

			if err := sync.SyncImage(ctx, image); err != nil {
				log.Error().
					Err(err).
					Str("source", image.Source).
					Msg("Failed to sync image")

				telemetry.ImageSyncErrors.Add(ctx, 1,
					metric.WithAttributes(
						attribute.KeyValue{
							Key:   "image",
							Value: attribute.StringValue(image.Source),
						},
						attribute.KeyValue{
							Key:   "error",
							Value: attribute.StringValue(err.Error()),
						},
					),
				)

				merr = multierr.Append(merr, err)

				if config.SyncMaxErrors.Int() > 0 {
					if len(multierr.Errors(merr)) >= config.SyncMaxErrors.Int() {
						return merr
					}
				}
			}
		}
	}

	return nil
}
