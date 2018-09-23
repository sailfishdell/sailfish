package main

import (
	"fmt"
	"os"

	log "github.com/inconshreveable/log15"
	"github.com/spf13/viper"
	mylog "github.com/superchalupa/sailfish/src/log"
)

// MyLogger is a centralized point for application logging that we will pass throughout the system
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

// New is the logger constructor which initializes an instance
func (l *MyLogger) New(ctx ...interface{}) mylog.Logger {
	return &MyLogger{
		Logger: l.Logger.New(ctx...),
	}
}

func (l *MyLogger) setupLogHandlersFromConfig(cfg *viper.Viper) {
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
	if path := cfg.GetString("main.log.FileName"); path != "" {
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
			l.Warn("type assertion failure for - module", "module", module, "ok", ok, "type", fmt.Sprintf("%T", module))
			continue
		}

		name, ok := module["name"].(string)
		if !ok {
			l.Warn("type assertion failure for - name", "name", name, "ok", ok, "raw", module["name"])
			continue
		}

		level, ok := module["level"].(string)
		if !ok {
			l.Warn("type assertion failure for - level", "level", level, "ok", ok, "raw", module["level"])
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
	l.SetHandler(
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
