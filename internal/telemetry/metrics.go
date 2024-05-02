package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("docker-sync")

var Errors = must(meter.Int64Counter("errors",
	metric.WithDescription("The total number of errors"),
))
