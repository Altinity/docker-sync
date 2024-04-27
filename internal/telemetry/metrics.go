package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("docker-sync")

// ReceivedBytes track the total number of bytes received.
var ReceivedBytes = must(meter.Int64Counter("received_bytes",
	metric.WithDescription("The total number of bytes received"),
	metric.WithUnit("By"),
))

// SentBytes track the total number of bytes sent.
var SentBytes = must(meter.Int64Counter("sent_bytes",
	metric.WithDescription("The total number of bytes sent"),
	metric.WithUnit("By"),
))
