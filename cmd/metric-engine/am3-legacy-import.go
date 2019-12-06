package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	eh "github.com/looplab/eventhorizon"

	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
)

const (
	LegacyDatabaseEvent eh.EventType = "LegacyDatabaseEvent"
)

func addAM3LegacyDatabaseFunctions(logger log.Logger, dbpath string, am3Svc *am3.Service, d *BusComponents) {
	database, err := sqlx.Open("sqlite3", dbpath)
	if err != nil {
		logger.Crit("Could not open legacy database", "err", err)
		return
	}

	// run sqlite with only one connection to avoid locking issues
	// If we run in WAL mode, you can only do one connection. Seems like a base
	// library limitation that's reflected up into the golang implementation.
	// SO: we will ensure that we have ONLY ONE GOROUTINE that does transactions
	// This isn't a terrible limitation as it is sort of what we want to do
	// anyways.
	database.SetMaxOpenConns(1)

	LegacyFactory, err := NewLegacyFactory(logger, database, d)
	if err != nil {
		logger.Crit("Error creating legacy integration", "err", err)
		database.Close()
		return
	}

	// We want to wait to start importing legacy data until we get the signal
	importWait := sync.WaitGroup{}
	importWait.Add(1)
	var start func()
	start = func() { importWait.Done(); start = func() {} }

	go func() {
		importWait.Wait()
		fmt.Printf("STARTING IMPORT LOOP\n")
		// do one immediate import
		d.GetBus().PublishEvent(context.Background(), eh.NewEvent(LegacyDatabaseEvent, "import", time.Now()))

		// then start a 10s loop
		legacyImportTimer := time.NewTicker(time.Duration(10) * time.Second)
		defer legacyImportTimer.Stop()
		for {
			select {
			case <-legacyImportTimer.C:
				d.GetBus().PublishEvent(context.Background(), eh.NewEvent(LegacyDatabaseEvent, "import", time.Now()))
			}
		}
	}()

	// Create a new Metric Report Definition, or update an existing one
	am3Svc.AddEventHandler("Import Legacy Metric Values", LegacyDatabaseEvent, func(event eh.Event) {
		command, ok := event.Data().(string)
		if !ok {
			logger.Crit("Legacy Metric DB message handler got an invalid data event", "event", event)
			command = "import"
		}

		switch command {
		case "start_timed_import":
			start()
			return

		// TODO: make this a per-table import so we can have separate timers per
		case "import":
			err = LegacyFactory.Import()
			if err != nil {
				logger.Crit("Failed to import legacy metrics", "err", err)
				return
			}
		}
	})
}
