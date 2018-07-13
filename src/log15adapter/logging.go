package commands

import (
	"fmt"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
	log "github.com/inconshreveable/log15"
	"github.com/spf13/viper"
	mylog "github.com/superchalupa/go-redfish/src/log"
)

// an adapter that hooks in log15

// MyLogger is a centralized point for application logging that we will pass throughout the system
type MyLogger struct {
	log.Logger
	logCfg   *viper.Viper
	logCfgMu sync.Mutex
}

// New is the logger constructor which initializes an instance
func (l *MyLogger) New(ctx ...interface{}) mylog.Logger {
	return &MyLogger{
		Logger: l.Logger.New(ctx...),
	}
}

func InitializeApplicationLogging(logCfgFile string) (logger *MyLogger) {

	logger = &MyLogger{
		Logger:   log.New(),
		logCfg:   viper.New(),
		logCfgMu: sync.Mutex{},
	}

	// Environment variables
	logger.logCfg.SetEnvPrefix("RFLOGGING")
	logger.logCfg.AutomaticEnv()

	// Configuration file
	if logCfgFile != "" {
		logger.logCfg.SetConfigFile(logCfgFile)
	} else {
		logger.logCfg.SetConfigName("redfish-logging")
		logger.logCfg.AddConfigPath(".")
		logger.logCfg.AddConfigPath("/etc/")
	}

	if err := logger.logCfg.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", logger.logCfg.ConfigFileUsed())
	} else {
		fmt.Fprintf(os.Stderr, "Could not read config file: %s\n", err)
	}

	setupLogHandlersFromConfig(logger)

	mylog.GlobalLogger = logger

	logger.logCfg.OnConfigChange(func(e fsnotify.Event) {
		logger.logCfgMu.Lock()
		defer logger.logCfgMu.Unlock()
		logger.Info("CONFIG file changed", "config_file", e.Name)
		setupLogHandlersFromConfig(logger)
	})
	logger.logCfg.WatchConfig()

	return
}

type LoggingConfig struct {
	Enabled         bool
	Level           string
	FileName        string
	PrintFunction   bool
	PrintFile       bool
	ModulesToEnable []map[string]string
}

func setupLogHandlersFromConfig(l *MyLogger) {

	LogConfig := []LoggingConfig{}

	err := l.logCfg.UnmarshalKey("logs", &LogConfig)
	if err != nil {
		log.Crit("Could not unmarshal logs key", "err", err)
	}

	if len(LogConfig) == 0 {
		log.Crit("Setting default config")
		LogConfig = []LoggingConfig{{
			Enabled:       true,
			Level:         "info",
			PrintFunction: true,
			PrintFile:     true,
		}}
	}

	topLevelHandlers := []log.Handler{}
	for _, onecfg := range LogConfig {
		if !onecfg.Enabled {
			continue
		}

		var outputHandler log.Handler
		switch path := onecfg.FileName; path {
		case "":
			fallthrough
		case "/dev/stderr":
			outputHandler = log.StreamHandler(os.Stderr, log.TerminalFormat())
		case "/dev/stdout":
			outputHandler = log.StreamHandler(os.Stdout, log.TerminalFormat())
		default:
			outputHandler = log.Must.FileHandler(path, log.LogfmtFormat())
		}

		if onecfg.PrintFile {
			outputHandler = log.CallerFileHandler(outputHandler)
		}
		if onecfg.PrintFunction {
			outputHandler = log.CallerFuncHandler(outputHandler)
		}

		wrappedOut := newOrHandler(outputHandler)

		handlers := []log.Handler{}

		loglvl, err := log.LvlFromString(onecfg.Level)
		if err == nil {
			handlers = append(handlers, log.LvlFilterHandler(loglvl, wrappedOut))
		}

		for _, m := range onecfg.ModulesToEnable {
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
    outMu   sync.RWMutex
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
    o.outMu.Lock()
	o.producedOutput = true
    o.outMu.Unlock()
	return o.outputHandler.Log(r)
}

func (o *orhandler) ORHandler(in ...log.Handler) log.Handler {
	return log.FuncHandler(func(r *log.Record) error {
        o.outMu.Lock()
		o.producedOutput = false
        o.outMu.Unlock()
		for _, h := range in {
			h.Log(r)
            o.outMu.RLock()
			if o.producedOutput {
                o.outMu.RUnlock()
				return nil
			}
            o.outMu.RUnlock()
		}
		return nil
	})
}
