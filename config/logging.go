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
)
