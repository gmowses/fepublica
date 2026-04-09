// Package logging sets up a zerolog logger based on config.
package logging

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// New returns a zerolog logger configured for the given level and format.
// format may be "json" (default) or "console".
func New(level, format, component string) zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339Nano

	var writer zerolog.ConsoleWriter
	var base zerolog.Logger
	if strings.EqualFold(format, "console") {
		writer = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
		base = zerolog.New(writer).With().Timestamp().Logger()
	} else {
		base = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}

	base = base.With().Str("component", component).Logger()

	switch strings.ToLower(level) {
	case "trace":
		base = base.Level(zerolog.TraceLevel)
	case "debug":
		base = base.Level(zerolog.DebugLevel)
	case "info":
		base = base.Level(zerolog.InfoLevel)
	case "warn":
		base = base.Level(zerolog.WarnLevel)
	case "error":
		base = base.Level(zerolog.ErrorLevel)
	default:
		base = base.Level(zerolog.InfoLevel)
	}

	return base
}
