package dockersync

import (
	"github.com/Altinity/docker-sync/config"
	"github.com/rs/zerolog/log"
)

// Reload refreshes the rate limiting configuration.
func Reload() {
	if reloadedKeys := config.Reload(); reloadedKeys != nil {
		for k := range reloadedKeys {
			if reloadedKeys[k].Error != nil {
				log.Error().
					Err(reloadedKeys[k].Error).
					Str("key", reloadedKeys[k].Key).
					Interface("oldValue", reloadedKeys[k].OldValue).
					Interface("newValue", reloadedKeys[k].NewValue).
					Msg("Failed to load configuration key, ignoring")
			} else if reloadedKeys[k].OldValue != nil {
				// Skip logging if the key was not previously set (i.e. it was loaded for the first time)
				/*
					  log.Info().
						Str("key", reloadedKeys[k].Key).
						Interface("oldValue", reloadedKeys[k].OldValue).
						Interface("newValue", reloadedKeys[k].NewValue).
						Msg("Loaded configuration key")
				*/
			}
		}
	}
}
