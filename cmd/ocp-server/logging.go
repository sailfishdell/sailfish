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
	mylog "github.com/superchalupa/go-redfish/src/log"

	eh "github.com/looplab/eventhorizon"
)

type Logger = mylog.Logger

// MyLogger is a simple event handler for logging all events.
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

	mylog.GlobalLogger = logger
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

func (logger *MyLogger) setupLogHandlersFromConfig(cfg *viper.Viper) {
	loglvl, err := log.LvlFromString(cfg.GetString("main.log.level"))
	if err != nil {
		log.Warn("Could not get desired main.log.level from configuration, falling back to default 'Info' level.", "error", err.Error(), "default", log.LvlInfo.String(), "got", cfg.GetString("main.log.level"))
		loglvl = log.LvlInfo
	}

	// optionally log to stderr, if enabled on CLI or in config
	// TODO: add cli option
	stderrHandler := log.DiscardHandler()
	if cfg.GetBool("main.log.EnableStderr") {
		stderrHandler = log.StreamHandler(os.Stderr, log.TerminalFormat())
	}

	// optionally log to file, if enabled on CLI or in config
	// TODO: add cli option
	fileHandler := log.DiscardHandler()
	if path := cfg.GetString("FileName"); path != "" {
		fileHandler = log.Must.FileHandler(path, log.LogfmtFormat())
	}

	outputHandler := log.MultiHandler(stderrHandler, fileHandler)

	// check for modules to enable
	moduleDebug := map[string]log.Lvl{}

	modulesToEnable, ok := cfg.Get("main.log.ModulesToEnable").([]interface{})
	if !ok {
		modulesToEnable = []interface{}{}
	}

	for _, m := range modulesToEnable {
		module, ok := m.(map[interface{}]interface{})
		if !ok {
			logger.Warn("type assertion failure for - module", "module", module, "ok", ok, "type", fmt.Sprintf("%T", module))
			continue
		}

		name, ok := module["name"].(string)
		if !ok {
			logger.Warn("type assertion failure for - name", "name", name, "ok", ok, "raw", module["name"])
			continue
		}

		level, ok := module["level"].(string)
		if !ok {
			logger.Warn("type assertion failure for - level", "level", level, "ok", ok, "raw", module["level"])
			continue
		}

		loglvl, err := log.LvlFromString(level)
		if err != nil {
			continue
		}

		moduleDebug[name] = loglvl
	}

	//
	// set up pipe to log to all of our configured outputs
	// first check gross log level and log if high enough, then check individual module list
	//
	logger.SetHandler(
		log.CallerFuncHandler(
			log.CallerFileHandler(
				log.FilterHandler(func(r *log.Record) bool {
					// check gross level first for speed for now. when we grow ability to supress on module basis, then move this to the end.
					if r.Lvl <= loglvl {
						return true
					}

					for i := 0; i < len(r.Ctx); i += 2 {
						if r.Ctx[i] == "module" {
							module, ok := r.Ctx[i+1].(string)
							if !ok {
								continue
							}

							if moduleLvl, ok := moduleDebug[module]; ok {
								if r.Lvl <= moduleLvl {
									return true
								}
							}
						}
					}
					return false
				}, outputHandler),
			)))
}
