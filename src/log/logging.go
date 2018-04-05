package log

import (
	"errors"
)

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

// don't use this. set it up in app, but ignore after
var GlobalLogger Logger

func MustLogger(module string) Logger {
	if GlobalLogger == nil {
		panic("Global Logger is not set up, cannot return logger.")
	}
	return GlobalLogger.New("module", module)
}

func GetLogger(module string) (Logger, error) {
	if GlobalLogger == nil {
		return nil, errors.New("Global Logger is not set up, cannot return logger.")
	}

	return GlobalLogger.New("module", module), nil
}
