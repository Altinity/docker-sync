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

func SyncTag(image *structs.Image, tag string, pullAuthName string, options ...remote.Option) error {
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

	syncTargets := make(map[string]remote.Option)
	if image.Auths == nil {
		image.Auths = make(map[string]remote.Option)
	}

	for _, target := range targets {
		tref, err := name.ParseReference(fmt.Sprintf("%s:%s", target, tag))
		if err != nil {
			merr = multierr.Append(merr, err)
			continue
		}

		pushAuth, ok := image.Auths[target]
		if !ok {
			pushAuth, _ = getAuth(image.GetRegistry(target), image.GetRepository(target))
			image.Auths[target] = pushAuth
		}

		if slices.Contains(image.MutableTags, tag) {
			syncTargets[target] = pushAuth
			continue
		}

		_, err = remote.Index(tref, pushAuth)
		if err != nil {
			if strings.Contains(err.Error(), "MANIFEST_UNKNOWN") {
				syncTargets[target] = pushAuth
				continue
			}
		}
	}

	if len(syncTargets) == 0 {
		log.Info().
			Str("source", image.Source).
			Str("tag", tag).
			Msg("Skipping tag sync, all targets are up to date")
		return nil
	}

	index, err := remote.Index(ref, options...)
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

	for target, pushAuth := range syncTargets {
		tref, err := name.ParseReference(fmt.Sprintf("%s:%s", target, tag))
		if err != nil {
			merr = multierr.Append(merr, err)
			continue
		}

		log.Info().
			Str("source", image.Source).
			Str("tag", tag).
			Strs("requiredPlatforms", expectedPlatforms).
			Str("target", target).
			Str("pullAuth", pullAuthName).
			Msg("Syncing tag")

		if err := remote.WriteIndex(tref, index, pushAuth); err != nil {
			merr = multierr.Append(merr, err)
		}
	}

	return merr
}
