package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/mux"
	log "github.com/inconshreveable/log15"
	"github.com/spf13/viper"
	mylog "github.com/superchalupa/go-redfish/src/log"

	eh "github.com/looplab/eventhorizon"
)

// MyLogger is a centralized point for application logging that we will pass throughout the system
type MyLogger struct {
	log.Logger
}

func initializeApplicationLogging() *MyLogger {

	cfg := viper.New()
	cfgMu := sync.Mutex{}

	// Environment variables
	cfg.SetEnvPrefix("RFLOGGING")
	cfg.AutomaticEnv()

	// Configuration file
	cfg.SetConfigName("redfish-logging")
	cfg.AddConfigPath(".")
	cfg.AddConfigPath("/etc/")
	if err := cfg.ReadInConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Could not read config file: %s\n", err)
	}

	logger := &MyLogger{
		Logger: log.New(),
	}
	logger.setupLogHandlersFromConfig(cfg)

	mylog.GlobalLogger = logger

	cfg.OnConfigChange(func(e fsnotify.Event) {
		cfgMu.Lock()
		defer cfgMu.Unlock()
		logger.Info("CONFIG file changed", "config_file", e.Name)
		logger.setupLogHandlersFromConfig(cfg)
	})
	cfg.WatchConfig()

	return logger
}

// New is the logger constructor which initializes an instance
func (l *MyLogger) New(ctx ...interface{}) mylog.Logger {
	return &MyLogger{
		Logger: l.Logger.New(ctx...),
	}
}

func (l *MyLogger) makeLoggingHTTPHandler(m http.Handler) http.HandlerFunc {
	// Simple HTTP request logging.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func(begin time.Time) {
			l.Info(
				"Processed http request",
				"source", r.RemoteAddr,
				"method", r.Method,
				"url", r.URL,
				"business_logic_time", time.Since(begin),
				"module", "http",
				"args", fmt.Sprintf("%#v", mux.Vars(r)),
			)
		}(time.Now())
		m.ServeHTTP(w, r)
	})
}

// Create a tiny logging middleware for the command handler.
func (l *MyLogger) makeLoggingCmdHandler(originalHandler eh.CommandHandler) eh.CommandHandler {
	return eh.CommandHandlerFunc(func(ctx context.Context, cmd eh.Command) error {
		l.Debug("Executed Command", "CMD", fmt.Sprintf("%v", cmd))
		return originalHandler.HandleCommand(ctx, cmd)
	})
}

// Notify implements the Notify method of the EventObserver interface.
func (l *MyLogger) Notify(ctx context.Context, event eh.Event) {
	l.Debug("Processed Event", "EVENT", event)
}

type LoggingConfig struct {
	Level           string
	FileName        string
	EnableStderr    bool
	PrintFunction   bool
	PrintFile       bool
	ModulesToEnable []map[string]string
}

func (l *MyLogger) setupLogHandlersFromConfig(cfg *viper.Viper) {

	LogConfig := []LoggingConfig{}

	err := cfg.UnmarshalKey("logs", &LogConfig)
	if err != nil {
		log.Crit("Could not unmarshal logs key", "err", err)
	}

	topLevelHandlers := []log.Handler{}
	for _, logcfg := range LogConfig {
		var outputHandler log.Handler
		switch path := logcfg.FileName; path {
		case "":
			fallthrough
		case "/dev/stderr":
			outputHandler = log.StreamHandler(os.Stderr, log.TerminalFormat())
		case "/dev/stdout":
			outputHandler = log.StreamHandler(os.Stdout, log.TerminalFormat())
		default:
			outputHandler = log.Must.FileHandler(path, log.LogfmtFormat())
		}

		if logcfg.PrintFile {
			outputHandler = log.CallerFileHandler(outputHandler)
		}
		if logcfg.PrintFunction {
			outputHandler = log.CallerFuncHandler(outputHandler)
		}

		wrappedOut := newOrHandler(outputHandler)

		handlers := []log.Handler{}

		loglvl, err := log.LvlFromString(logcfg.Level)
		if err == nil {
			handlers = append(handlers, log.LvlFilterHandler(loglvl, wrappedOut))
		}

		for _, m := range logcfg.ModulesToEnable {
			name, ok := m["name"]
			if !ok {
				l.Warn("Nonexistent name for config", "m", m)
				continue
			}
			handler := log.MatchFilterHandler("module", name, wrappedOut)

			level, ok := m["level"]
			if ok {
				loglvl, err := log.LvlFromString(level)
				if err == nil {
					handler = log.LvlFilterHandler(loglvl, handler)
				} else {
					l.Warn("Could not parse level for config", "m", m)
				}
			} else {
				l.Warn("Nonexistent level for config", "m", m)
			}

			handlers = append(handlers, handler)
		}

		topLevelHandlers = append(topLevelHandlers, wrappedOut.ORHandler(handlers...))
	}

	l.SetHandler(log.MultiHandler(topLevelHandlers...))
}

type orhandler struct {
	producedOutput bool
	outputHandler  log.Handler
}

func newOrHandler(out log.Handler) *orhandler {
	o := &orhandler{
		producedOutput: false,
		outputHandler:  out,
	}
	return o
}

func (o *orhandler) Log(r *log.Record) error {
	o.producedOutput = true
	return o.outputHandler.Log(r)
}

func (o *orhandler) ORHandler(in ...log.Handler) log.Handler {
	return log.FuncHandler(func(r *log.Record) error {
		o.producedOutput = false
		for _, h := range in {
			h.Log(r)
			if o.producedOutput {
				return nil
			}
		}
		return nil
	})
}
