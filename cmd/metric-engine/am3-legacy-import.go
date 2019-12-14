package main

import (
	"context"
	"fmt"
	"strings"
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

type LegacyDatabseEventData struct {
	TableName string
}

func addAM3LegacyDatabaseFunctions(logger log.Logger, dbpath string, am3Svc *am3.Service, d *BusComponents) {
	database, err := sqlx.Open("sqlite3", dbpath)
	if err != nil {
		logger.Crit("Could not open legacy database", "err", err)
		return
	}

	// legacy db not opened in WAL mode... in fact should be read-only, so this isn't really necessary, I think
	database.SetMaxOpenConns(1)

	LegacyFactory, err := NewLegacyFactory(logger, database, d)
	if err != nil {
		logger.Crit("Error creating legacy integration", "err", err)
		database.Close()
		return
	}

	err = LegacyFactory.PrepareAll()
	if err != nil {
		logger.Crit("Error preparing legacy queries", "err", err)
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
		// This is a separate goroutine. CANNOT call into the LegacyFactory or it will data race!
		// do one immediate import
		d.GetBus().PublishEvent(context.Background(), eh.NewEvent(LegacyDatabaseEvent, "import_all", time.Now()))

		// then start a 10s loop
		legacyImportTimer := time.NewTicker(time.Duration(10) * time.Second)
		defer legacyImportTimer.Stop()
		for {
			select {
			case <-legacyImportTimer.C:
				d.GetBus().PublishEvent(context.Background(), eh.NewEvent(LegacyDatabaseEvent, "import_all", time.Now()))
			}
		}
	}()

	// Create a new Metric Report Definition, or update an existing one
	am3Svc.AddEventHandler("Import Legacy Metric Values", LegacyDatabaseEvent, func(event eh.Event) {
		command, ok := event.Data().(string)
		if !ok {
			logger.Crit("Legacy Metric DB message handler got an invalid data event", "event", event, "eventdata", event.Data())
			command = "import"
		}

		switch {
		case command == "start_timed_import":
			start()
			return

		case strings.HasPrefix(command, "import:"):
			more, err := LegacyFactory.Import(command[7:])
			if err != nil {
				logger.Crit("Import failed over legacy tables", "import", command, "err", err)
				return
			}
			if more {
				d.GetBus().PublishEvent(context.Background(), eh.NewEvent(LegacyDatabaseEvent, command, time.Now()))
			}
			return

		case command == "import_all":
			err := LegacyFactory.IterLegacyTables(func(s string) error {
				return d.GetBus().PublishEvent(context.Background(), eh.NewEvent(LegacyDatabaseEvent, "import:"+s, time.Now()))
			})
			if err != nil {
				logger.Crit("Iteraton failed over legacy tables", "err", err)
				return
			}
		default:
			logger.Crit("GOT A COMMAND THAT I CANT HANDLE", "command", command)
		}
	})
}
