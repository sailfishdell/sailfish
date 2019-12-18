package udb

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	eh "github.com/looplab/eventhorizon"

	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
)

const (
	UDBDatabaseEvent eh.EventType = "UDBDatabaseEvent"
)

type UDBDatabseEventData struct {
	TableName string
}

type BusComponents interface {
	GetBus() eh.EventBus
}

func RegisterAM3(logger log.Logger, dbpath string, am3Svc *am3.Service, d BusComponents) {
	database, err := sqlx.Open("sqlite3", dbpath)
	if err != nil {
		logger.Crit("Could not open udb database", "err", err)
		return
	}

	// udb db not opened in WAL mode... in fact should be read-only, so this isn't really necessary, but might as well
	database.SetMaxOpenConns(1)

	UDBFactory, err := NewUDBFactory(logger, database, d)
	if err != nil {
		logger.Crit("Error creating udb integration", "err", err)
		database.Close()
		return
	}

	UDBFactory.PrepareAll()
	if err != nil {
		logger.Crit("Error preparing udb queries", "err", err)
		return
	}

	// for now, trigger automatic imports on a periodic basis
	go func() {
		importTicker := time.NewTicker(5 * time.Second)
		time.Sleep(1 * time.Second)
		defer importTicker.Stop()
		for {
			select {
			case <-importTicker.C:
				d.GetBus().PublishEvent(context.Background(), eh.NewEvent(UDBDatabaseEvent, "import:Sensor", time.Now()))
			}
		}
	}()

	// Create a new Metric Report Definition, or update an existing one
	am3Svc.AddEventHandler("Import UDB Metric Values", UDBDatabaseEvent, func(event eh.Event) {
		command, ok := event.Data().(string)
		if !ok {
			logger.Crit("UDB Metric DB message handler got an invalid data event", "event", event, "eventdata", event.Data())
			return
		}

		switch {
		case strings.HasPrefix(command, "import:"):
			parts := strings.Split(command, ":")

			err := UDBFactory.Import(parts[1], parts[2:]...)
			if err != nil {
				logger.Crit("Import failed over udb tables", "import", command, "err", err)
				return
			}

		default:
			logger.Crit("GOT A COMMAND THAT I CANT HANDLE", "command", command)
		}
	})
}
