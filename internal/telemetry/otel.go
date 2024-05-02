package telemetry

import (
	"context"
	"fmt"

	"github.com/Altinity/docker-sync/config"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// Start initializes the telemetry system and starts the metrics server if enabled.
func Start(ctx context.Context) error {
	logger := log.With().Str("component", "telemetry").Logger()
	ctx = logger.WithContext(ctx)

	// Set up the resource that identifies the service.
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithHost(),
		resource.WithAttributes(
			attribute.String("service", "docker-sync"),
		),
	)
	if err != nil {
		return err
	}

	// Set up meter provider.
	meterProvider, err := newMeterProvider(res)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("Error creating meter provider")
		return err
	}
	defer func() {
		if err := meterProvider.Shutdown(ctx); err != nil {
			logger.Error().
				Err(err).
				Msg("Error shutting down meter provider")
		}
	}()
	otel.SetMeterProvider(meterProvider)

	// Start the metrics server if enabled.
	serverErrChan := make(chan error, 1)

	if config.TelemetryMetricsExporter.String() == "prometheus" {
		go func() {
			serverErrChan <- startPrometheusServer(ctx)
		}()
	}

	// Wait for the server to stop or the context to be canceled.
	for {
		select {
		case err := <-serverErrChan:
			if err != nil {
				logger.Error().
					Err(err).
					Msg("Error in telemetry server")
				return err
			}
		case <-ctx.Done():
			logger.Info().Msg("Stopping telemetry")
			return nil
		}
	}
}

// newMeterProvider creates a new meter provider based on the configured metrics exporter.
func newMeterProvider(res *resource.Resource) (*metric.MeterProvider, error) {
	var meterProvider *metric.MeterProvider

	switch config.TelemetryMetricsExporter.String() {
	case "prometheus":
		exporter, err := prometheus.New(prometheus.WithNamespace("docker-sync"))
		if err != nil {
			return nil, err
		}
		meterProvider = metric.NewMeterProvider(metric.WithReader(exporter), metric.WithResource(res))
	case "stdout":
		exporter, err := stdoutmetric.New()
		if err != nil {
			return nil, err
		}
		meterProvider = metric.NewMeterProvider(
			metric.WithReader(
				metric.NewPeriodicReader(exporter,
					metric.WithInterval(config.TelemetryMetricsStdoutInterval.Duration()))),
			metric.WithResource(res))
	default:
		return nil, fmt.Errorf("unknown metrics exporter: %s", config.TelemetryMetricsExporter.String())
	}

	return meterProvider, nil
}
