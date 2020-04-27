package log

import ()

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
