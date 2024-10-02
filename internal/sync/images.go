package sync

import (
	"context"
	"strings"
	"time"

	"github.com/Altinity/docker-sync/internal/telemetry"
	"github.com/Altinity/docker-sync/structs"
	"github.com/cenkalti/backoff/v4"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"
)

func SyncImage(ctx context.Context, image *structs.Image) error {
	var merr error

	pullAuth, pullAuthName := getAuth(image.GetSourceRegistry(), image.GetSourceRepository())

	tags, err := image.GetTags(pullAuth)
	if err != nil {
		return err
	}

	for _, tag := range tags {
		if err := backoff.Retry(func() error {
			if err := SyncTag(image, tag, pullAuthName, pullAuth); err != nil {
				if strings.Contains(err.Error(), "HAP429") || strings.Contains(err.Error(), "TOOMANYREQUESTS") {
					log.Warn().
						Str("source", image.Source).
						Msg("Rate limited by registry, backing off")
					return err
				}

				return backoff.Permanent(err)
			}

			return nil
		}, backoff.NewExponentialBackOff(
			backoff.WithInitialInterval(1*time.Minute),
		)); err != nil {
			errs := multierr.Errors(err)
			if len(errs) > 0 {
				telemetry.Errors.Add(ctx, int64(len(errs)),
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
				log.Error().
					Errs("errors", errs).
					Msg("Failed to sync tag")

				merr = multierr.Append(merr, err)
				continue
			}
		}
	}

	return merr
}
