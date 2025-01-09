package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("docker-sync")

var Errors = must(meter.Int64Counter("errors",
	metric.WithDescription("Total number of errors"),
))

var Syncs = must(meter.Int64Counter("syncs",
	metric.WithDescription("Total number of syncs"),
))

var UploadedBytes = must(meter.Int64Counter("uploaded_bytes",
	metric.WithDescription("Total number of bytes uploaded"),
))
