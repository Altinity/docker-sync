package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/Altinity/docker-sync/config"
	"github.com/Altinity/docker-sync/internal/telemetry"
	"github.com/Altinity/docker-sync/structs"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"
)

var lastSyncMap = make(map[string]time.Time)

func SyncImage(ctx context.Context, image *structs.Image) error {
	var merr error

	pullAuth, pullAuthName := getAuth(image.GetSourceRegistry(), image.GetSourceRepository())

	tags, err := image.GetTags(pullAuth)
	if err != nil {
		return err
	}

	for _, tag := range tags {
		k := fmt.Sprintf("%s:%s", image.Source, tag)
		if t, ok := lastSyncMap[k]; ok {
			if time.Since(t) < config.SyncMinInterval.Duration() {
				log.Info().
					Str("source", image.GetSource()).
					Str("tag", tag).
					Msg("Skipping tag sync due to minimum interval")
				continue
			}
		}

		if serr := SyncTag(image, tag, pullAuthName, pullAuth); serr != nil {
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
		lastSyncMap[k] = time.Now()
	}

	return merr
}
