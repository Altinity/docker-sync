package sync

import (
	"github.com/Altinity/docker-sync/structs"
	"github.com/rs/zerolog/log"
	"go.uber.org/multierr"
)

func SyncImage(image *structs.Image) error {
	var merr error

	tags, err := image.GetTags()
	if err != nil {
		return err
	}

	for _, tag := range tags {
		if serr := SyncTag(image, tag); serr != nil {
			errs := multierr.Errors(serr)
			if len(errs) > 0 {
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
