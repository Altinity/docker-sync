package sync

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Altinity/docker-sync/structs"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/types"
	"github.com/rs/zerolog/log"
)

func getSourceTags(ctx context.Context, image *structs.Image, srcCtx *types.SystemContext, srcRef types.ImageReference) ([]string, error) {
	var srcTags []string
	var err error

	var allTags []string

	if len(image.Tags) > 0 {
		for _, tag := range image.Tags {
			if tag == "@semver" {
				if allTags == nil {
					allTags, err = docker.GetRepositoryTags(ctx, srcCtx, srcRef)
					if err != nil {
						return nil, err
					}
				}

				for _, t := range allTags {
					if isSemVerTag(t) {
						srcTags = append(srcTags, t)
					}
				}
			} else if strings.Contains(tag, "*") {
				if allTags == nil {
					allTags, err = docker.GetRepositoryTags(ctx, srcCtx, srcRef)
					if err != nil {
						return nil, err
					}
				}

				for _, t := range allTags {
					if match, err := filepath.Match(tag, t); err == nil && match {
						srcTags = append(srcTags, t)
					}
				}
			} else {
				srcTags = append(srcTags, tag)
			}
		}
	} else {
		srcTags, err = docker.GetRepositoryTags(ctx, srcCtx, srcRef)
		if err != nil {
			return nil, err
		}
	}

	// Remove duplicate tags
	slices.Sort(srcTags)

	return slices.Compact(srcTags), nil
}

func getDstTags(ctx context.Context, image *structs.Image) ([]string, error) {
	var dstTags []string

	for _, dst := range image.Targets {
		switch getRepositoryType(dst) {
		case S3CompatibleRepository:
			fields := strings.Split(dst, ":")

			tags, err := listS3Tags(ctx, dst, fields)
			if err != nil {
				return nil, err
			}

			if len(tags) > 0 {
				log.Info().
					Str("image", image.Source).
					Str("target", dst).
					Int("tags", len(tags)).
					Msg("Found destination tags")

				dstTags = append(dstTags, tags...)
			}

			continue
		case OCIRepository:
			dstRef, err := docker.ParseReference(fmt.Sprintf("//%s", dst))
			if err != nil {
				return nil, err
			}

			dstAuth, dstAuthName := getSkopeoAuth(ctx, image.GetRegistry(dst), image.GetRepository(dst))

			dstCtx := &types.SystemContext{
				DockerAuthConfig: dstAuth,
			}

			tags, err := docker.GetRepositoryTags(ctx, dstCtx, dstRef)
			if err != nil {
				return nil, err
			}

			if len(tags) > 0 {
				log.Info().
					Str("image", image.Source).
					Str("target", dst).
					Int("tags", len(tags)).
					Str("auth", dstAuthName).
					Msg("Found destination tags")

				for _, tag := range tags {
					dstTags = append(dstTags, fmt.Sprintf("%s:%s", dst, tag))
				}
			}
		}
	}

	// Remove duplicate tags
	slices.Sort(dstTags)

	return slices.Compact(dstTags), nil
}
