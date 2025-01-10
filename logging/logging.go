package logging

import (
	"fmt"
	"time"

	"github.com/Altinity/docker-sync/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
)

// NewLogger instantiates and returns a new *zerolog.Logger.
func NewLogger() *zerolog.Logger {
	// Set the global duration field unit to milliseconds
	zerolog.DurationFieldUnit = time.Second
	// Set the global time field format to RFC3339
	zerolog.TimeFieldFormat = time.RFC3339

	// Declare a variable to hold the multi-level writer
	var multi zerolog.LevelWriter

	var wr diode.Writer

	output := newLogWriter()

	// Create a new diode writer with a buffer size of 1000 and a flush interval of 10 milliseconds
	wr = diode.NewWriter(output, 1000, 10*time.Millisecond, func(missed int) {
		// If any messages are dropped, print the number of dropped messages
		fmt.Printf("dropped %d messages", missed)
	})

	// Assign the diode writer to the multi-level writer
	multi = zerolog.MultiLevelWriter(wr)

	// Create a new logger with the multi-level writer, add a timestamp to each log message
	l := zerolog.New(multi).With().Timestamp()

	logger := l.Logger()

	// Set the global log level to Info
	if lvl, err := zerolog.ParseLevel(config.LoggingLevel.String()); err == nil {
		logger = logger.Level(lvl)
	} else {
		logger = logger.Level(zerolog.InfoLevel)
	}

	// Return the logger
	return &logger
}
