package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"

	"github.com/fsnotify/fsnotify"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	//	log "github.com/superchalupa/go-redfish/src/log"
)

func main() {
	var cfgMgrMu sync.Mutex
	cfgMgr := viper.New()
	if err := cfgMgr.BindPFlags(flag.CommandLine); err != nil {
		fmt.Fprintf(os.Stderr, "Could not bind viper flags: %s\n", err)
	}
	// Environment variables
	cfgMgr.SetEnvPrefix("RE")
	cfgMgr.AutomaticEnv()

	// Configuration file
	cfgMgr.SetConfigName("redfish-events")
	cfgMgr.AddConfigPath(".")
	cfgMgr.AddConfigPath("/etc/")
	if err := cfgMgr.ReadInConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Could not read config file: %s\n", err)
	}

	// Defaults
	// put any viper config defaults here, none yet, use this as an example
	// cfgMgr.SetDefault("session.timeout", 10)

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	intr := make(chan os.Signal, 1)
	signal.Notify(intr, os.Interrupt)

	logger := initializeApplicationLogging(cfgMgr)

	type configHandler interface {
		ConfigChangeHandler()
	}

	cfgMgr.OnConfigChange(func(e fsnotify.Event) {
		cfgMgrMu.Lock()
		defer cfgMgrMu.Unlock()
		logger.Info("CONFIG file changed", "config_file", e.Name)
		for _, fn := range logger.ConfigChangeHooks {
			fn()
		}
		// you should put a hook here to re-run config handling for any sub modules....
		// even better: have sub modules use their own viper config
	})
	cfgMgr.WatchConfig()

	// Put our code here!
	_ = ctx // eliminate unused var warning on ctx since we will definitely use it later
	fmt.Println("Hello world")

	logger.Debug("Hello world")
	SdNotify("READY=1")

	// wait until we get an interrupt (CTRL-C)
	<-intr
	cancel()
	logger.Warn("Bye!", "module", "main")
}
