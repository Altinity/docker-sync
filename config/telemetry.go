package config

var (
	// region Telemetry.

	// TelemetryEnabled indicates whether telemetry is enabled.
	TelemetryEnabled = NewKey("telemetry.enabled",
		WithDefaultValue(false),
		WithValidBool())

	// TelemetryMetricsExporter specifies the metrics exporter used for telemetry.
	TelemetryMetricsExporter = NewKey("telemetry.metrics.exporter",
		WithDefaultValue("prometheus"),
		WithAllowedStrings([]string{"prometheus", "stdout"}))

	// TelemetryMetricsPrometheusAddress specifies the network address for the Prometheus.
	TelemetryMetricsPrometheusAddress = NewKey("telemetry.metrics.prometheus.address",
		WithDefaultValue("127.0.0.1:9090"),
		WithValidNetHostPort())

	// TelemetryMetricsPrometheusPath specifies the path for the Prometheus metrics endpoint.
	TelemetryMetricsPrometheusPath = NewKey("telemetry.metrics.prometheus.path",
		WithDefaultValue("/metrics"),
		WithValidURI())

	// TelemetryMetricsStdoutInterval specifies the sampling interval for sending metrics to stdout.
	TelemetryMetricsStdoutInterval = NewKey("telemetry.metrics.stdout.interval",
		WithDefaultValue("5s"),
		WithValidDuration())
	// endregion.
)
