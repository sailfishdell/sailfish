package log

import (
	"errors"
)

// Logger type is a lowest-common-denominator logging interface that can be adapted to work with many different logging subsystems
type Logger interface {
	// New returns a new Logger that has this logger's context plus the given context
	New(ctx ...interface{}) Logger

	// Log a message at the given level with context key/value pairs
	Debug(msg string, ctx ...interface{})
	Info(msg string, ctx ...interface{})
	Warn(msg string, ctx ...interface{})
	Error(msg string, ctx ...interface{})
	Crit(msg string, ctx ...interface{})

	// at the point in time where we need the lazy evaluation feature of log15, add a helper interface here:
	//Lazy()
}

// GlobalLogger is for the application main() to set up as a base
// TODO: this should be unexported and a helper function set up to set this
var GlobalLogger Logger

// MustLogger will return a logger or will panic
func MustLogger(module string) Logger {
	if GlobalLogger == nil {
		panic("Global Logger is not set up, cannot return logger.")
	}
	return GlobalLogger.New("module", module)
}

// GetLogger returns a logger or an error
func GetLogger(module string) (Logger, error) {
	if GlobalLogger == nil {
		return nil, errors.New("global Logger is not set up, cannot return logger")
	}

	return GlobalLogger.New("module", module), nil
}
