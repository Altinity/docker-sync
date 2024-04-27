package config

var (
	// region Logging

	// LoggingFormat defines the logging format used for log messages. Allowed values are "json" and "text".
	LoggingFormat = NewKey("logging.format",
		WithDefaultValue("text"),
		WithAllowedStrings([]string{"json", "text"}))

	// LoggingColors specifies whether log messages are displayed with color-coded
	// output. This only applies when LoggingFormat is set to "text".
	LoggingColors = NewKey("logging.colors",
		WithDefaultValue(true),
		WithValidBool())

	// LoggingTimeFormat specifies the time format used for log messages. The default is 15:04:05.
	LoggingTimeFormat = NewKey("logging.timeFormat",
		WithDefaultValue("15:04:05"))

	// LoggingOutput specifies the destination where log messages created by the
	// application are sent. It defaults to stdout.
	LoggingOutput = NewKey("logging.output",
		WithDefaultValue("stdout"))

	// LoggingLevel specifies the minimum severity a log message must meet to be
	// recorded.
	LoggingLevel = NewKey("logging.level",
		WithDefaultValue("INFO"),
		WithAllowedStrings([]string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL", "PANIC", "DISABLED"}))
	// endregion

	// region Telemetry

	// TelemetryEnabled indicates whether telemetry is enabled.
	TelemetryEnabled = NewKey("telemetry.enabled",
		WithDefaultValue(false),
		WithValidBool())

	// TelemetryMetricsExporter specifies the metrics exporter used for telemetry.
	TelemetryMetricsExporter = NewKey("telemetry.metrics.exporter",
		WithDefaultValue("prometheus"),
		WithAllowedStrings([]string{"prometheus", "stdout"}))

	// TelemetryMetricsPrometheusAddress specifies the network address for the Prometheus
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
	// endregion

	// region Repositories

	// Repositories specifies the repositories to use for pulling and pushing images.
	Repositories = NewKey("repositories",
		WithDefaultValue([]map[string]interface{}{
			{
				"name": "Docker Hub",
				"url":  "docker.io",
				"auth": map[string]interface{}{
					"username": "",
					"password": "",
					"token":    "",
					"helper":   "docker-credential-desktop",
				},
			},
		}),
		WithValidRepositories())
	// endregion

	// region Images

	// Images specifies the images to synchronize.
	Images = NewKey("images",
		WithDefaultValue([]map[string]interface{}{
			{
				"source": "docker.io/library/ubuntu",
				"targets": []string{
					"docker.io/library/ubuntu:latest",
				},
			},
		}),
		WithValidImages())

	// endregion
)
