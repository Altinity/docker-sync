package dockersync

import (
	"context"

	"github.com/Altinity/docker-sync/config"
	"github.com/Altinity/docker-sync/internal/sync"
	"github.com/Altinity/docker-sync/internal/telemetry"
	"github.com/rs/zerolog/log"
	"go.uber.org/multierr"
)

func Run(ctx context.Context) error {
	var merr error

	if config.TelemetryEnabled.Bool() {
		go func() {
			if err := telemetry.Start(ctx); err != nil {
				log.Error().
					Err(err).
					Msg("Failed to start telemetry")
			}
		}()
	}

	images := config.Images.Images()

	for k := range images {
		image := images[k]

		if err := sync.SyncImage(image); err != nil {
			merr = multierr.Append(merr, err)
		}
	}

	errs := multierr.Errors(merr)
	if len(errs) > 0 {
		return merr
	}

	return nil
}
