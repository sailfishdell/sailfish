package watchdog

import (
	"context"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/event"
)

type eventHandlingService interface {
	AddEventHandler(string, eh.EventType, func(eh.Event))
}

const (
	WatchdogEvent        = eh.EventType("Watchdog")
	watchdogsPerInterval = 3
)

type WatchdogEventData struct {
	Seq int
}

type busComponents interface {
	GetBus() eh.EventBus
}

// StartWatchdogHandling will attach event handlers to ping systemd watchdog from AM3 for watchdog events
func StartWatchdogHandling(logger log.Logger, am3Svc eventHandlingService, d busComponents) error {
	s := 0
	eh.RegisterEventData(WatchdogEvent, func() eh.EventData { s++; return &WatchdogEventData{Seq: s} })

	var sd sdNotifier
	sd, err := NewSdnotify()
	if err != nil {
		logger.Warn("Running using simulation SD_NOTIFY", "err", err)
		sd = SimulateSdnotify()
	}

	// add the watchdog handling to the awesome mapper. meaning that the entire
	// event bus infra has to be working and functional for watchdog to be
	// pinged.
	am3Svc.AddEventHandler("Ping Systemd Watchdog", WatchdogEvent, func(eh.Event) {
		sd.SDNotify("WATCHDOG=1")
	})

	interval := sd.GetIntervalUsec()
	if interval == 0 {
		interval = 30000000
	}
	interval /= watchdogsPerInterval

	// set up separate thread that periodically publishes watchdog events on the bus
	go generateWatchdogEvents(logger, d.GetBus(), time.Duration(interval)*time.Microsecond)
	sd.SDNotify("READY=1")

	return nil
}

func generateWatchdogEvents(logger log.Logger, bus eh.EventBus, interval time.Duration) {
	logger.Info("Setting up watchdog.", "interval-in-milliseconds", interval)
	// endless loop generating and responding to watchdog events
	watchdogTicker := time.NewTicker(interval)
	defer watchdogTicker.Stop()
	for {
		select {
		case <-watchdogTicker.C:
			data, err := eh.CreateEventData(WatchdogEvent)
			if err != nil {
				logger.Crit("error creating event", "err", err)
				continue
			}
			evt := event.NewSyncEvent(WatchdogEvent, data, time.Now())
			evt.Add(1)
			err = bus.PublishEvent(context.Background(), evt)
			if err != nil {
				logger.Crit("error publishing event", "err", err)
				continue
			}
			evt.Wait()
		}
	}
}
