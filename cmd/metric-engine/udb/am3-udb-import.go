package udb

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/looplab/event"

	"github.com/superchalupa/sailfish/cmd/metric-engine/telemetry-db"

	log "github.com/superchalupa/sailfish/src/log"
)

const (
	UDBDatabaseEvent eh.EventType = "UDBDatabaseEvent"
	UDBChangeEvent   eh.EventType = "UDBChangeEvent"
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
	_, err = database.Exec(`
		-- ensure nothing we do will ever modify the source
		PRAGMA query_only = 1;
		-- should be set in connection string, but just in case:
		PRAGMA busy_timeout = 1000;
	  -- don't ever run sync() or friends
		PRAGMA synchronous = off;
		PRAGMA       journal_mode  = off;
		PRAGMA udbdm.journal_mode  = off;
		PRAGMA udbsm.journal_mode  = off;
	  -- these seem to increase memory usage, so disable until we get good values for these
		-- PRAGMA cache_size = 0;
		-- PRAGMA udbdm.cache_size = 0;
		-- PRAGMA PRAGMA udbsm.cache_size = 0;
		-- PRAGMA mmap_size = 0;
		`)
	if err != nil {
		panic("Could not set up initial UDB database parameters: " + err.Error())
	}

	// we have only one thread doing updates, so one connection should be
	// fine. keeps sqlite from opening new connections un-necessarily
	database.SetMaxOpenConns(1)

	UDBFactory, err := NewUDBFactory(logger, database, d, cfg)
	if err != nil {
		logger.Crit("Error creating udb integration", "err", err)
		database.Close()
		return
	}

	go handleUDBNotifyPipe(logger, cfg, d)

	// This is the event to trigger UDB imports. We will only attach it after a second to let all startup settle before we start processing imports
	go func() {
		time.Sleep(1 * time.Second)
		am3Svc.AddEventHandler("Import UDB Metric Values", telemetry.DatabaseMaintenance, func(event eh.Event) {
			// TODO: get smarter about this. We ought to calculate time until next report and set a timer for that
			UDBFactory.IterUDBTables(func(name string, meta UDBMeta) error {
				UDBFactory.ConditionalImport(name, meta, true)
				return nil
			})
		})
	}()

	am3Svc.AddEventHandler("UDB Change Notification", UDBChangeEvent, func(event eh.Event) {
		notify, ok := event.Data().(*ChangeNotify)
		if !ok {
			logger.Crit("UDB Change Notifier message handler got an invalid data event", "event", event, "eventdata", event.Data())
			return
		}
		UDBFactory.DBChanged(strings.ToLower(notify.Database), strings.ToLower(notify.Table))
	})
}

type ChangeNotify struct {
	Database  string
	Table     string
	Rowid     int64
	Operation int64
}

func handleUDBNotifyPipe(logger log.Logger, cfg *viper.Viper, d BusComponents) {
	pipePath := cfg.GetString("main.udbnotifypipe")

	// Data format we get:
	//    DB                      TBL                  ROWID     operationid
	// ||DMLiveObjectDatabase.db|TblNic_Port_Stats_Obj|167445167|23||

	err := makeFifo(pipePath, 0660)
	if err != nil && !os.IsExist(err) {
		logger.Warn("Error creating UDB pipe", "err", err)
	}

	file, err := os.OpenFile(pipePath, os.O_CREATE, os.ModeNamedPipe)
	if err != nil {
		logger.Crit("Error opening UDB pipe", "err", err)
	}

	defer file.Close()

	// The reader of the named pipe gets an EOF when the last writer exits. To
	// avoid this, we'll simply open it ourselves for writing and never close it.
	// This will ensure the pipe stays around forever without eof.

	nullWriter, err := os.OpenFile(pipePath, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		logger.Crit("Error opening UDB pipe for (placeholder) write", "err", err)
	}

	// this function doesn't return (on purpose), so this defer won't do much. That's ok, we'll keep it in case we change things around in the future
	defer nullWriter.Close()

	n := &ChangeNotify{}
	split := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF {
			return 0, nil, io.EOF
		}
		start := bytes.Index(data, []byte("||"))
		if start == -1 {
			// didnt find starting ||, skip over everything
			return len(data), nil, nil
		}

		end := bytes.Index(data[start+2:], []byte("||"))
		if end == -1 {
			// didnt find ending ||
			return 0, nil, nil
		}

		// adjust 'end' here to take into account that we indexed off the start+2
		// of the data array
		fields := bytes.Split(data[start+2:end+start+2], []byte("|"))
		if len(fields) != 4 {
			n.Database = ""
			n.Table = ""
			n.Rowid = 0
			n.Operation = 0
			// skip over starting || plus any intervening data, leave the trailing || as potential start of next record
			return start + end + 2, []byte("s"), nil
		}

		n.Database = string(fields[0])
		n.Table = string(fields[1])
		n.Rowid, _ = strconv.ParseInt(string(fields[2]), 10, 64)
		n.Operation, _ = strconv.ParseInt(string(fields[3]), 10, 64)

		// consume the whole thing
		return start + 2 + end + 2, []byte("t"), nil
	}

	// give everything a chance to settle before we start processing
	time.Sleep(1 * time.Second)
	fmt.Printf("STARTING UDB NOTIFY PIPE HANDLER\n")

	s := bufio.NewScanner(file)
	s.Split(split)
	for s.Scan() {
		if s.Text() == "t" {
			// publish change notification
			evt := event.NewSyncEvent(UDBChangeEvent, n, time.Now())
			evt.Add(1)
			d.GetBus().PublishEvent(context.Background(), evt)
			evt.Wait()
			// new struct for the next notify so we dont have data races while other goroutines process the struct above
			n = &ChangeNotify{}
		}
	}
}
