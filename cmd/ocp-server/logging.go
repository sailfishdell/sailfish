package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	log "github.com/inconshreveable/log15"
	"github.com/spf13/viper"

	eh "github.com/looplab/eventhorizon"
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

// Logger is a simple event handler for logging all events.
type MyLogger struct {
	log.Logger
	ConfigChangeHooks []func()
}

func initializeApplicationLogging(cfg *viper.Viper) *MyLogger {
	logger := &MyLogger{
		Logger: log.New(),
	}
	logger.ConfigChangeHooks = append(logger.ConfigChangeHooks, func() { logger.setupLogHandlersFromConfig(cfg) })
	logger.setupLogHandlersFromConfig(cfg)
	return logger
}

func NewLogger(ctx ...interface{}) Logger {
	return &MyLogger{
		Logger: log.New(ctx...),
	}
}

func (l *MyLogger) New(ctx ...interface{}) Logger {
	return &MyLogger{
		Logger: l.Logger.New(ctx...),
	}
}

func (l *MyLogger) makeLoggingHttpHandler(m http.Handler) http.HandlerFunc {
	// Simple HTTP request logging.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func(begin time.Time) {
			l.Debug(
				"Processed http request",
				"source", r.RemoteAddr,
				"method", r.Method,
				"url", r.URL,
				"business_logic_time", time.Since(begin),
				"args", fmt.Sprintf("%#v", mux.Vars(r)),
			)
		}(time.Now())
		m.ServeHTTP(w, r)
	})
}

// Create a tiny logging middleware for the command handler.
func (l *MyLogger) makeLoggingCmdHandler(originalHandler eh.CommandHandler) eh.CommandHandler {
	return eh.CommandHandlerFunc(func(ctx context.Context, cmd eh.Command) error {
		l.Debug("Executed Command", "CMD", cmd)
		return originalHandler.HandleCommand(ctx, cmd)
	})
}

// Notify implements the Notify method of the EventObserver interface.
func (l *MyLogger) Notify(ctx context.Context, event eh.Event) {
	l.Debug("Processed Event", "EVENT", event)
}

func (logger *MyLogger) setupLogHandlersFromConfig(cfg *viper.Viper) {
	loglvl, err := log.LvlFromString(cfg.GetString("main.logLevel"))
	if err != nil {
		log.Warn("Could not get desired main.logLevel from configuration, falling back to default 'Info' level.", "error", err.Error(), "default", log.LvlInfo.String(), "got", cfg.GetString("main.logLevel"))
		loglvl = log.LvlInfo
	}

	// optionally log to stderr, if enabled on CLI or in config
	// TODO: add cli option
	stderrHandler := log.DiscardHandler()
	if cfg.GetBool("main.logEnableStderr") {
		stderrHandler = log.StreamHandler(os.Stderr, log.TerminalFormat())
	}

	// optionally log to file, if enabled on CLI or in config
	// TODO: add cli option
	fileHandler := log.DiscardHandler()
	if path := cfg.GetString("logToFile"); path != "" {
		fileHandler = log.Must.FileHandler(path, log.LogfmtFormat())
	}

	// set up pipe to log to all of our configured outputs
	logger.SetHandler(
		log.LvlFilterHandler(
			loglvl,
			log.MultiHandler(
				stderrHandler,
				fileHandler,
			),
		),
	)
}
