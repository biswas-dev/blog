package main

import (
	"os"

	"github.com/rs/zerolog"
)

var logger zerolog.Logger

func initLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	return logger
}
