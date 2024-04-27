package logging

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// ReloadGlobalLogger reinitializes the global logger instance with updated configurations.
func ReloadGlobalLogger() {
	log.Logger = *NewLogger()
	zerolog.DefaultContextLogger = &log.Logger
}
