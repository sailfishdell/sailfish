package log15adapter

import (
	"fmt"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
	log "github.com/inconshreveable/log15"
	"github.com/spf13/viper"
	mylog "github.com/superchalupa/sailfish/src/log"
)

// an adapter that hooks in log15

// MyLogger is a centralized point for application logging that we will pass throughout the system
type MyLogger struct {
	log.Logger
}

// New is the logger constructor which initializes an instance
func (l *MyLogger) New(ctx ...interface{}) mylog.Logger {
	return &MyLogger{
		Logger: l.Logger.New(ctx...),
	}
}

func Make() *MyLogger {
	return &MyLogger{
		Logger: log.New(),
	}
}

func InitializeApplicationLogging(logCfgFile string) (logger *MyLogger) {
	// Environment variables
	logCfg := viper.New()
	logCfg.SetEnvPrefix("RFLOGGING")
	logCfg.AutomaticEnv()

	// Configuration file
	if logCfgFile != "" {
		logCfg.SetConfigName(logCfgFile)
	} else {
		logCfg.SetConfigName("redfish-logging")
	}

	// set search paths for configs
	logCfg.AddConfigPath(".")
	logCfg.AddConfigPath("/etc/")

	if err := logCfg.ReadInConfig(); err == nil {
		fmt.Println("log15adapter: Using config file:", logCfg.ConfigFileUsed())
	} else {
		fmt.Fprintf(os.Stderr, "log15adapter: Could not read config file: %s\n", err)
	}

	logger = Make()
	logger.SetupLogHandlersFromConfig(logCfg)
	logger.ActivateConfigWatcher(logCfg)

	return
}

func (l *MyLogger) ActivateConfigWatcher(logCfg *viper.Viper) {
	logCfg.OnConfigChange(func(e fsnotify.Event) {
		l.Info("CONFIG file changed", "config_file", e.Name)
		l.SetupLogHandlersFromConfig(logCfg)
		// free up memory after
		//logCfg.Reset()
	})
	logCfg.WatchConfig()
}

type LoggingConfig struct {
	Enabled         bool
	Level           string
	FileName        string
	PrintFunction   bool
	PrintFile       bool
	ModulesToEnable []map[string]string
}

func (l *MyLogger) SetupLogHandlersFromConfig(logCfg *viper.Viper) {
	LogConfig := []LoggingConfig{}

	err := logCfg.UnmarshalKey("logs", &LogConfig)
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
			handler := MatchFilterHandler("module", name, wrappedOut)

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

	if mylog.GlobalLogger == nil {
		mylog.GlobalLogger = l
	}
}

type orhandler struct {
	outMu          sync.RWMutex
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

type Record = log.Record
type Handler = log.Handler

// search through all the keys for a match
func MatchFilterHandler(key string, value interface{}, h Handler) Handler {
	return log.FilterHandler(func(r *Record) (pass bool) {
		switch key {
		case r.KeyNames.Lvl:
			return r.Lvl == value
		case r.KeyNames.Time:
			return r.Time == value
		case r.KeyNames.Msg:
			return r.Msg == value
		}

		for i := 0; i < len(r.Ctx); i += 2 {
			if r.Ctx[i] == key {
				if r.Ctx[i+1] == value {
					return true
				}
			}
		}
		return false
	}, h)
}
