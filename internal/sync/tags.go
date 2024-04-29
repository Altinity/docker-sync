package sync

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Altinity/docker-sync/structs"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/rs/zerolog/log"
	"go.uber.org/multierr"
)

func SyncTag(image *structs.Image, tag string) error {
	var merr error

	expectedPlatforms := image.GetRequiredPlatforms()
	if len(expectedPlatforms) == 0 {
		expectedPlatforms = append(expectedPlatforms, "linux/amd64")
	}

	targets := image.GetTargets()

	ref, err := name.ParseReference(fmt.Sprintf("%s:%s", image.Source, tag))
	if err != nil {
		return err
	}

	pullAuth, pullAuthName := getAuth(image.GetSourceRegistry(), image.GetSourceRepository())
	index, err := remote.Index(ref, pullAuth)
	if err != nil {
		if strings.Contains(err.Error(), "unsupported MediaType") {
			log.Warn().
				Str("source", ref.Name()).
				Str("tag", tag).
				Msg("Skipping legacy v1 manifest")
		}

		return nil
	}

	idxManifest, err := index.IndexManifest()
	if err != nil {
		return err
	}

	var foundPlatforms []string
	for k := range idxManifest.Manifests {
		platform := fmt.Sprintf("%s/%s", idxManifest.Manifests[k].Platform.OS, idxManifest.Manifests[k].Platform.Architecture)

		if !slices.Contains(expectedPlatforms, platform) {
			continue
		}

		foundPlatforms = append(foundPlatforms, platform)
	}

	for _, platform := range expectedPlatforms {
		if !slices.Contains(foundPlatforms, platform) {
			merr = multierr.Append(merr, fmt.Errorf("image %s:%s is missing platform %s", image.Source, tag, platform))
		}
	}

	for _, target := range targets {
		tref, err := name.ParseReference(fmt.Sprintf("%s:%s", target, tag))
		if err != nil {
			merr = multierr.Append(merr, err)
			continue
		}

		pushAuth, pushAuthName := getAuth(image.GetRegistry(target), image.GetRepository(target))

		log.Info().
			Str("source", image.Source).
			Str("tag", tag).
			Strs("requiredPlatforms", expectedPlatforms).
			Str("target", target).
			Str("pullAuth", pullAuthName).
			Str("pushAuth", pushAuthName).
			Msg("Syncing tag")

		if err := remote.WriteIndex(tref, index, pushAuth); err != nil {
			merr = multierr.Append(merr, err)
		}
	}

	return merr
}
