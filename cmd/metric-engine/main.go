package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"
	"github.com/superchalupa/sailfish/src/looplab/eventbus"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"

	"github.com/superchalupa/sailfish/src/log15adapter"
)

type shutdowner interface {
	Shutdown(context.Context) error
}

type BusComponents struct {
	EventBus       eh.EventBus
	EventWaiter    *eventwaiter.EventWaiter
	EventPublisher eh.EventPublisher
}

func (d *BusComponents) GetBus() eh.EventBus                 { return d.EventBus }
func (d *BusComponents) GetWaiter() *eventwaiter.EventWaiter { return d.EventWaiter }
func (d *BusComponents) GetPublisher() eh.EventPublisher     { return d.EventPublisher }

func main() {
	flag.StringSliceP("listen", "l", []string{}, "Listen address.  Formats: (http:[ip]:nn, https:[ip]:port)")

	cfgMgr := viper.New()
	if err := cfgMgr.BindPFlags(flag.CommandLine); err != nil {
		fmt.Fprintf(os.Stderr, "Could not bind viper flags: %s\n", err)
		panic(fmt.Sprintf("Could not bind viper flags: %s", err))
	}
	// Environment variables
	cfgMgr.SetEnvPrefix("ME")
	cfgMgr.AutomaticEnv()

	// Configuration file
	cfgMgr.SetConfigName("metric-engine")
	cfgMgr.AddConfigPath("/etc/")
	cfgMgr.AddConfigPath(".")
	if err := cfgMgr.ReadInConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Could not read config file: %s\n", err)
		panic(fmt.Sprintf("Could not read config file: %s", err))
	}

	// Defaults
	cfgMgr.SetDefault("listen", []string{"https::8443"})
	cfgMgr.SetDefault("main.server_name", "idrac")

	flag.Parse()

	logger := log15adapter.Make()
	logger.SetupLogHandlersFromConfig(cfgMgr)

	ctx, cancel := context.WithCancel(context.Background())
	intr := make(chan os.Signal, 1)
	signal.Notify(intr, os.Interrupt)
	go func() {
		// wait until <CTRL>-C
		<-intr
		logger.Crit("INTERRUPTED, Cancelling...")
		cancel()
	}()

	d := &BusComponents{
		EventBus:       eventbus.NewEventBus(),
		EventPublisher: eventpublisher.NewEventPublisher(),
		EventWaiter:    eventwaiter.NewEventWaiter(eventwaiter.SetName("Main"), eventwaiter.NoAutoRun),
	}

	d.EventBus.AddHandler(eh.MatchAny(), d.EventPublisher)
	d.EventPublisher.AddObserver(d.EventWaiter)
	go d.GetWaiter().Run()

	setup(ctx, logger, cfgMgr, d)
	h := starthttp(logger, cfgMgr, d)

	// wait until everything is done
	<-ctx.Done()
	shutdown()
	h.shutdown()

	logger.Warn("Bye!", "module", "main")
}

func init() {
	go func() {
		t := time.Tick(time.Second * 30)
		for {
			<-t
			debug.FreeOSMemory()
		}
	}()
}
