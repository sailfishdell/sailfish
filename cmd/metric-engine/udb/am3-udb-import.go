package udb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	log "github.com/superchalupa/sailfish/src/log"
)

const (
	UDBDatabaseEvent eh.EventType = "UDBDatabaseEvent"
)

type BusComponents interface {
	GetBus() eh.EventBus
}

type EventHandlingService interface {
	AddEventHandler(string, eh.EventType, func(eh.Event))
}

func RegisterAM3(logger log.Logger, cfg *viper.Viper, am3Svc EventHandlingService, d BusComponents) {
	database, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		logger.Crit("Could not open udb database", "err", err)
		return
	}

	// attach UDB db
	attach := "Attach '" + cfg.GetString("main.udbdatabasepath") + "' as udbdm"
	fmt.Println(attach)
	_, err = database.Exec(attach)
	if err != nil {
		logger.Crit("Could not attach UDB database", "attach", attach, "err", err)
		return
	}

	// attach SHM db
	attach = "Attach '" + cfg.GetString("main.shmdatabasepath") + "' as udbsm"
	fmt.Println(attach)
	_, err = database.Exec(attach)
	if err != nil {
		logger.Crit("Could not attach SM database", "attach", attach, "err", err)
		return
	}

	// we have a separate goroutine for this, so we should be safe to busy-wait
	_, err = database.Exec("PRAGMA query_only = 1")
	_, err = database.Exec("PRAGMA busy_timeout = 1000")
	//_, err = database.Exec("PRAGMA cache_size = 0")
	//_, err = database.Exec("PRAGMA udbdm.cache_size = 0")
	//_, err = database.Exec("PRAGMA udbsm.cache_size = 0")
	//_, err = database.Exec("PRAGMA mmap_size = 0")
	_, err = database.Exec("PRAGMA synchronous = off")
	_, err = database.Exec("PRAGMA       journal_mode  = off")
	_, err = database.Exec("PRAGMA udbdm.journal_mode  = off")
	_, err = database.Exec("PRAGMA udbsm.journal_mode  = off")

	// udb db not opened in WAL mode... in fact should be read-only, so this isn't really necessary, but might as well
	database.SetMaxOpenConns(1)

	UDBFactory, err := NewUDBFactory(logger, database, d, cfg)
	if err != nil {
		logger.Crit("Error creating udb integration", "err", err)
		database.Close()
		return
	}

	// for now, trigger automatic imports on a periodic basis (5s for now, we can up to 1s later to catch power stuff)
	go func() {
		importTicker := time.NewTicker(5 * time.Second)
		defer importTicker.Stop()
		for {
			select {
			case <-importTicker.C:
				d.GetBus().PublishEvent(context.Background(), eh.NewEvent(UDBDatabaseEvent, "import", time.Now()))
			}
		}
	}()

	// This is the event to trigger UDB imports
	am3Svc.AddEventHandler("Import UDB Metric Values", UDBDatabaseEvent, func(event eh.Event) {
		command, ok := event.Data().(string)
		if !ok {
			logger.Crit("UDB Metric DB message handler got an invalid data event", "event", event, "eventdata", event.Data())
			return
		}

		switch {
		// Trigger one specific import unconditionally
		case strings.HasPrefix(command, "import:"):
			parts := strings.Split(command, ":")

			err := UDBFactory.Import(parts[1], parts[2:]...)
			if err != nil {
				logger.Crit("Import failed over udb tables", "import", command, "err", err)
				return
			}

		case command == "import":
			UDBFactory.IterUDBTables(func(name string) error {
				UDBFactory.ConditionalImport(name)
				return nil
			})

		default:
			logger.Crit("GOT A COMMAND THAT I CANT HANDLE", "command", command)
		}
	})
}
