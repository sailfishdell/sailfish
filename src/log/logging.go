package log

import (
	"errors"
)

// Logger type is a lowest-common-denominator logging interface that can be adapted to work with many different logging subsystems
type Logger interface {
	Debug(msg string, ctx ...interface{})
	Info(msg string, ctx ...interface{})
	Warn(msg string, ctx ...interface{})
	Error(msg string, ctx ...interface{})
	Crit(msg string, ctx ...interface{})
}

type newer interface {
	New(ctx ...interface{}) Logger
}

func With(logger Logger, keyvals ...interface{}) Logger {
	n, ok := logger.(newer)
	if ok {
		return n.New(keyvals...)
	}
	return logger
}

// GlobalLogger is for the application main() to set up as a base
// TODO: this should be unexported and a helper function set up to set this
var GlobalLogger Logger

// MustLogger will return a logger or will panic
func MustLogger(module string) Logger {
	if GlobalLogger == nil {
		panic("Global Logger is not set up, cannot return logger.")
	}
	return With(GlobalLogger, "module", module)
}

// GetLogger returns a logger or an error
func GetLogger(module string) (Logger, error) {
	if GlobalLogger == nil {
		return nil, errors.New("global Logger is not set up, cannot return logger")
	}

	return With(GlobalLogger, "module", module), nil
}
