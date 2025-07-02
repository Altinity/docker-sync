package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("docker-sync")

var TagSyncErrors = must(meter.Int64Counter("tag_sync_errors",
	metric.WithDescription("Total number of tag sync errors"),
))

var ImageSyncErrors = must(meter.Int64Counter("image_sync_errors",
	metric.WithDescription("Total number of image sync errors"),
))

var PurgeErrors = must(meter.Int64Counter("purge_errors",
	metric.WithDescription("Total number of purge errors"),
))

var Pushes = must(meter.Int64Counter("pushes",
	metric.WithDescription("Total number of pushes"),
))

var UploadedBytes = must(meter.Int64Counter("uploaded_bytes",
	metric.WithDescription("Total number of bytes uploaded"),
))

var DownloadedBytes = must(meter.Int64Counter("downloaded_bytes",
	metric.WithDescription("Total number of bytes downloaded"),
))

var MonitoredImages = must(meter.Int64Gauge("monitored_images",
	metric.WithDescription("Total number of monitored images"),
))

var MonitoredTags = must(meter.Int64Gauge("monitored_tags",
	metric.WithDescription("Total number of monitored tags"),
))
