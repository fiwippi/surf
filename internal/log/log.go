package log

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05"})
}

func Logger(lvl zerolog.Level) *zerolog.Logger {
	l := log.Logger.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05"}).Level(lvl)
	return &l
}

func Error() *zerolog.Event {
	return log.Error()
}

func Warn() *zerolog.Event {
	return log.Warn()
}

func Fatal() *zerolog.Event {
	return log.Fatal()
}

func Info() *zerolog.Event {
	return log.Info()
}

func Debug() *zerolog.Event {
	return log.Debug()
}

func With() zerolog.Context {
	return log.With()
}
