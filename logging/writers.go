package logging

import (
	"fmt"
	"io"
	"os"

	"github.com/Altinity/docker-sync/config"
	"github.com/rs/zerolog"
)

// newLogWriter selects an [io.Writer] for logging based on the application's
// configuration, determining the appropriate output destination and format. For
// text logs, it enhances the output with features like color coding and tabular
// formatting for console outputs.
func newLogWriter() io.Writer {
	output := config.LoggingOutput.String()
	format := config.LoggingFormat.String()

	switch format {
	case "json":
		switch output {
		case "stdout":
			return os.Stdout
		case "stderr":
			return os.Stderr
		default:
			f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Println("[ERROR] Failed to open log file:", err)
				fmt.Println("[WARN] Defaulting to stdout")

				return os.Stdout
			}
			return f
		}

	case "text":
		switch output {
		case "stdout":
			return consoleWriter()
		case "stderr":
			c := consoleWriter()
			c.Out = os.Stderr
			return c
		default:
			f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Println("[ERROR] Failed to open log file:", err)
				fmt.Println("[WARN] Defaulting to stdout")

				return os.Stdout
			}
			return f
		}
	}

	// Only warn if invalid values are set
	if output != "" && format != "" {
		fmt.Println("[WARN] Unknown log format / output combination, defaulting to stdout")
	}

	return os.Stdout
}

// consoleWriter creates and returns a [zerolog.ConsoleWriter] that formats log
// messages for display in console environments.
func consoleWriter() zerolog.ConsoleWriter {
	// Create a new console writer that outputs to the standard output
	writer := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: config.LoggingTimeFormat.String(),
		NoColor:    !config.LoggingColors.Bool(),
	}

	// Set the order of the parts in the log message
	writer.PartsOrder = []string{
		zerolog.TimestampFieldName,
		zerolog.CallerFieldName,
		zerolog.LevelFieldName,
		zerolog.MessageFieldName,
	}

	// Define the format of the message part
	/*
		writer.FormatMessage = func(i interface{}) string {
			return fmt.Sprintf("%s", i)
		}
	*/

	// Return the configured console writer
	return writer
}
