package sync

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/Altinity/docker-sync/internal/telemetry"
	"github.com/Altinity/docker-sync/structs"
	"github.com/cenkalti/backoff/v4"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/types"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func checkRateLimit(err error) error {
	if err == nil {
		return nil
	}

	if strings.Contains(err.Error(), "HAP429") || strings.Contains(err.Error(), "TOOMANYREQUESTS") {
		log.Warn().
			Msg("Rate limited by registry, backing off")
		return err
	}

	return backoff.Permanent(err)
}

func SyncImage(ctx context.Context, image *structs.Image) error {
	log.Info().
		Str("image", image.Source).
		Strs("targets", image.Targets).
		Msg("Syncing image")

	srcRef, err := docker.ParseReference(fmt.Sprintf("//%s", image.Source))
	if err != nil {
		return err
	}
	image.SrcRef = srcRef

	srcAuth, srcAuthName := getSkopeoAuth(ctx, image.GetSourceRegistry(), image.GetSourceRepository())
	srcCtx := &types.SystemContext{
		DockerAuthConfig: srcAuth,
	}

	srcTags, err := getSourceTags(ctx, image, srcCtx, srcRef)
	if err != nil {
		return err
	}

	if len(srcTags) == 0 {
		log.Warn().
			Str("image", image.Source).
			Str("auth", srcAuthName).
			Msg("No source tags found, skipping image")

		return nil
	}

	telemetry.MonitoredTags.Record(ctx, int64(len(srcTags)),
		metric.WithAttributes(
			attribute.KeyValue{
				Key:   "image",
				Value: attribute.StringValue(image.Source),
			},
		),
	)

	log.Info().
		Str("image", image.Source).
		Str("auth", srcAuthName).
		Int("tags", len(srcTags)).
		Msg("Found source tags")

	// Get all tags from targets
	dstTags, err := getDstTags(ctx, image)
	if err != nil {
		return err
	}

	// Sync tags
	for _, tag := range srcTags {
		if slices.Contains(image.IgnoredTags, tag) {
			log.Info().
				Str("image", image.Source).
				Str("tag", tag).
				Msg("Skipping ignored tag")
			continue
		}

		syncTag(ctx, image, tag, dstTags)
	}

	// Purge
	purge(ctx, image, srcTags, dstTags)

	return nil
}
