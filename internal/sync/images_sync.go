package sync

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/Altinity/docker-sync/internal/telemetry"
	"github.com/Altinity/docker-sync/structs"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func syncTag(ctx context.Context, image *structs.Image, tag string, dstTags []string) {
	// Initialize telemetry for the tag
	telemetry.TagSyncErrors.Add(ctx, 0,
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

	var actualDsts []string

	// Determine the actual destinations for the tag
	for _, dst := range image.Targets {
		if !slices.Contains(image.MutableTags, tag) && // Explicitly mutable tags
			slices.Contains(dstTags, fmt.Sprintf("%s:%s", dst, tag)) && // Check if the tag already exists in the target
			!slices.ContainsFunc(image.MutableTags, func(t string) bool {
				match, _ := filepath.Match(t, tag)
				return match
			}) && // Check if the tag matches any mutable tag patterns
			!slices.Contains(image.MutableTags, "*") { // All tags are mutable if "*" is present
			continue
		}

		actualDsts = append(actualDsts, dst)
	}

	if len(actualDsts) == 0 {
		log.Debug().
			Str("image", image.Source).
			Str("tag", tag).
			Msg("Tag already exists in all targets, skipping")

		return
	}

	log.Info().
		Str("image", image.Source).
		Str("tag", tag).
		Strs("targets", image.Targets).
		Msg("Syncing tag")

	for _, dst := range actualDsts {
		if err := push(ctx, image, dst, tag); err != nil {
			log.Error().
				Err(err).
				Str("image", image.Source).
				Str("tag", tag).
				Msg("Failed to sync tag")

			telemetry.TagSyncErrors.Add(ctx, 1,
				metric.WithAttributes(
					attribute.KeyValue{
						Key:   "image",
						Value: attribute.StringValue(image.Source),
					},
					attribute.KeyValue{
						Key:   "tag",
						Value: attribute.StringValue(tag),
					},
					attribute.KeyValue{
						Key:   "error",
						Value: attribute.StringValue(err.Error()),
					},
				),
			)
		} else {
			telemetry.Pushes.Add(ctx, 1,
				metric.WithAttributes(
					attribute.KeyValue{
						Key:   "image",
						Value: attribute.StringValue(image.Source),
					},
					attribute.KeyValue{
						Key:   "tag",
						Value: attribute.StringValue(tag),
					},
					attribute.KeyValue{
						Key:   "target",
						Value: attribute.StringValue(dst),
					},
				),
			)
		}
	}
}
