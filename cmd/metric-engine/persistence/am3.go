package persistence

import (
	"context"
	"encoding/json"
	"golang.org/x/xerrors"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/looplab/event"

	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"
	"github.com/superchalupa/sailfish/cmd/metric-engine/telemetry"
	"github.com/superchalupa/sailfish/src/fileutils"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
)

type eventHandler interface {
	AddEventHandler(string, eh.EventType, func(eh.Event)) error
	AddMultiHandler(string, eh.EventType, func(eh.Event)) error
}

const (
	persistChanLen   = 5 // may move to yaml
	persistTimeDelay = time.Second * 2
	getTimeout       = persistTimeDelay / 2
	SAVE             = 0
	DELETE           = 1
	mrdDir           = "mrd/"
	mdDir            = "md/"
	trigDir          = "trigger/"
)

type Response interface {
	StreamResponse(io.Writer) error
}

type urigetter interface {
	GetURI() string
}

type busIntf interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
}

type persistMeta struct {
	URI        string
	actionType int // save (0), delete(1)
	fileLoc    string
	bkupLoc    string
	recovType  eh.EventType
}

func Handler(ctx context.Context, logger log.Logger, cfg *viper.Viper, am3Svc eventHandler, d busIntf) {

	// setup viper defaults
	ch := make(chan persistMeta, persistChanLen)
	pmrdDir := cfg.GetString("persistence.topsavedir") + mrdDir
	pROmrdDir := cfg.GetString("persistence.topimportonlydir") + mrdDir

	for _, i := range []struct {
		am3name    string
		eventType  eh.EventType
		saveDir    string
		bkupDir    string
		recovType  eh.EventType
		jsonAction int
	}{
		{"Add MRD", telemetry.AddMRDResponseEvent, pmrdDir, "", "", SAVE},
		{"Update MRD", telemetry.UpdateMRDResponseEvent, pmrdDir, "", "", SAVE},
		{"Delete MRD", telemetry.DeleteMRDResponseEvent, pmrdDir, pROmrdDir, telemetry.AddMRDCommandEvent, DELETE},
	} {
		err := am3Svc.AddEventHandler(i.am3name, i.eventType, MakeHandlerAddToChan(logger, ch, i.saveDir, i.bkupDir, i.recovType, i.jsonAction))
		if err != nil {
			logger.Warn("P-Handler", "err", err.Error())
		}
	}

	// go thread that persist data in the background
	go persistJSONwithDelay(logger, d, ch)
}

func uriGetter(event eh.Event) string {
	g, ok := event.Data().(urigetter)
	if !ok {
		return ""
	}
	return g.GetURI()
}

func MakeHandlerAddToChan(logger log.Logger, ch chan persistMeta, saveDir string, bkupDir string, recovType eh.EventType, jsonAction int) func(eh.Event) {
	return func(event eh.Event) {
		URI := uriGetter(event)

		ch <- persistMeta{
			URI,
			jsonAction,
			saveDir,
			bkupDir,
			recovType,
		}
	}
}

func persistJSONwithDelay(logger log.Logger, bus busIntf, ch chan persistMeta) {
	timer := time.NewTimer(persistTimeDelay)
	toUpdate := map[string]persistMeta{}
	trigger := true

	for {
		select {
		case meta := <-ch:
			toUpdate[meta.URI] = meta

			if !trigger {
				timer = time.NewTimer(persistTimeDelay)
				trigger = true
			}

		case <-timer.C:
			toSave := map[string]persistMeta{}

			for URI, meta := range toUpdate {
				toSave[URI] = persistMeta{
					actionType: meta.actionType,
					fileLoc:    meta.fileLoc,
					bkupLoc:    meta.bkupLoc,
					recovType:  meta.recovType,
				}
				delete(toUpdate, URI)
			}
			trigger = false
			go persistJSON(logger, bus, toSave)
		}
	}
}

// is it possible to launch two of these at once? shouldn't happen..
func persistJSON(logger log.Logger, bus busIntf, saveMeta map[string]persistMeta) {
	for URI, metaData := range saveMeta {
		fileName := getFileName(URI)
		switch metaData.actionType {
		case SAVE:
			updateJSON(logger, bus, URI, metaData.fileLoc+fileName)
		case DELETE:
			deleteAndRestoreJSON(logger, bus.GetBus(), metaData.recovType, metaData.fileLoc+fileName, metaData.bkupLoc+fileName)

		default:
			logger.Warn("Unknown Action", "Action", metaData.actionType, "URI", URI)
		}
	}
}

func getFileName(uri string) string {
	urlSplit := strings.Split(uri, "/")
	name := urlSplit[len(urlSplit)-1]
	return name + ".json"
}

func deleteAndRestoreJSON(logger log.Logger, bus eh.EventBus, recovType eh.EventType, fileToDelete string, permFile string) {
	os.Remove(fileToDelete)
	if fileutils.FileExists(permFile) {
		jsonstr, err := ioutil.ReadFile(permFile)
		if err != nil {
			logger.Warn("deleteAndRestoreJSON", "can not read json", permFile)
			return
		}

		eventData, err := eh.CreateEventData(recovType)
		if err != nil {
			logger.Warn("deleteAndRestoreJSON", "can not create event", recovType)
			return
		}

		err = json.Unmarshal(jsonstr, eventData)
		if err != nil {
			logger.Warn("deleteAndRestoreJSON", "can not unmarshal json to eventData")
			return
		}

		err = bus.PublishEvent(context.Background(), eh.NewEvent(recovType, eventData, time.Now()))
		if err != nil {
			logger.Warn("deleteAndRestoreJSON", "restore did not succeed for", fileToDelete)
		}
	}
}

func updateJSON(logger log.Logger, bus busIntf, uri string, permFile string) {
	// ensure that this dies eventually
	cmd, evt, err := metric.CommandFactory(telemetry.GenericGETCommandEvent)()
	if err != nil {
		return
	}

	cmd.(*telemetry.GenericGETCommandData).URI = uri

	outputFile, err := os.OpenFile(permFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		logger.Warn("ERROR: %s\n", err.Error())
		return
	}
	defer outputFile.Close()
	cmd.SetResponseHandlers(nil, nil, outputFile)

	ctx, cancel := context.WithTimeout(context.Background(), getTimeout)
	defer cancel()
	l := eventwaiter.NewListener(ctx, logger, bus.GetWaiter(), cmd.ResponseWaitFn())
	defer l.Close()

	l.Name = "JSON Persistence"
	err = bus.GetBus().PublishEvent(ctx, evt)
	if err != nil {
		logger.Warn("updateJSON", "ERROR! %s", err.Error())
	}

	_, err = l.Wait(ctx)
	if err != nil {
		logger.Warn("updateJSON", "err", err)
	}
}

func Import(logger log.Logger, cfg *viper.Viper, bus eh.EventBus) error {
	importdir := cfg.GetString("persistence.topimportonlydir")
	savedir := cfg.GetString("persistence.topsavedir")

	var persistentImportDirs = []struct {
		name      string
		dir       string
		recovDir  string
		eventType eh.EventType
	}{
		{"MetricDefinition", importdir + mdDir, "", telemetry.AddMDCommandEvent},
		{"MetricReportDefinition", savedir + mrdDir, importdir + mrdDir, telemetry.AddMRDCommandEvent},
		{"Trigger", importdir + trigDir, "", telemetry.CreateTriggerCommandEvent},
	}

	// strategy: this process *has* to succeed for md and triggers. If it does not, return error and we panic.
	//            MRDs have a recovery mechanism and won't trigger a panic unless the recovery mechanism fails
	// TODO: need to think of a process to clear out errors and automatically. Theoretically should not happen, but would be fatal if it did.
	for _, pi := range persistentImportDirs {
		files, err := ioutil.ReadDir(pi.dir)
		if err != nil {
			return xerrors.Errorf("Error reading import dir: %s to import %s: %w", pi.dir, pi.name, err)
		}

		for _, file := range files {
			filename := pi.dir + file.Name()
			fileRecovPath := pi.recovDir + file.Name()

			jsonstr, err := ioutil.ReadFile(filename)
			if err != nil {
				return xerrors.Errorf("Error reading %s import file(%s): %w", filename, filename, err)
			}

			eventData, err := eh.CreateEventData(pi.eventType)
			if err != nil {
				return xerrors.Errorf("Couldnt create %s event for file(%s) import. Should never happen: %w", filename, filename, err)
			}

			err = json.Unmarshal(jsonstr, eventData)
			if err != nil && len(pi.recovDir) != 0 {
				if fileutils.FileExists(fileRecovPath) {
					deleteAndRestoreJSON(logger, bus, pi.eventType, filename, fileRecovPath)
					continue
				}
			} else if err != nil {
				return xerrors.Errorf("Malformed %s JSON file(%s), error unmarshalling JSON: %w", pi.name, filename, err)
			}

			evt := event.NewSyncEvent(pi.eventType, eventData, time.Now())
			evt.Add(1)
			err = bus.PublishEvent(context.Background(), evt)
			if err != nil {
				return xerrors.Errorf("Error publishing %s event for file(%s) import: %w", pi.name, filename, err)
			}
			evt.Wait()
		}
	}

	return nil
}
