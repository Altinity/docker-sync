package sync

import (
	"context"

	"github.com/Altinity/docker-sync/internal/telemetry"
	"github.com/Altinity/docker-sync/structs"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"
)

func SyncImage(ctx context.Context, image *structs.Image) error {
	var merr error

	tags, err := image.GetTags()
	if err != nil {
		return err
	}

	for _, tag := range tags {
		if serr := SyncTag(image, tag); serr != nil {
			errs := multierr.Errors(serr)
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
			}

			merr = multierr.Append(merr, serr)
			continue
		}
	}

	return merr
}
