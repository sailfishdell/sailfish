package persistence

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"
	tele "github.com/superchalupa/sailfish/cmd/metric-engine/telemetry"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
)

type eventHandler interface {
	AddEventHandler(string, eh.EventType, func(eh.Event)) error
}

const (
	persistChanLen   = 5 // may move to yaml
	persistTimeDelay = time.Second * 2
	getTimeout       = persistTimeDelay / 2
	SAVE             = 0
	DELETE           = 1
	mrdDir           = "mrd/"
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
}

func Handler(ctx context.Context, logger log.Logger, cfg *viper.Viper, am3Svc eventHandler, d busIntf) {
	ch := make(chan persistMeta, persistChanLen)
	pmrdDir := cfg.GetString("persistence.topsavedir") + mrdDir
	pROmrdDir := cfg.GetString("persistence.topimportonlydir") + mrdDir

	for _, i := range []struct {
		am3name    string
		eventType  eh.EventType
		saveDir    string
		bkupDir    string
		jsonAction int
	}{
		{"Add MRD", tele.AddMRDResponseEvent, pmrdDir, "", SAVE},
		{"Update MRD", tele.UpdateMRDResponseEvent, pmrdDir, "", SAVE},
		{"Delete MRD", tele.DeleteMRDResponseEvent, pmrdDir, pROmrdDir, DELETE},
	} {
		err := am3Svc.AddEventHandler(i.am3name, i.eventType, MakeHandlerAddToChan(logger, ch, i.saveDir, i.bkupDir, i.jsonAction))
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

func MakeHandlerAddToChan(logger log.Logger, ch chan persistMeta, saveDir string, bkupDir string, jsonAction int) func(eh.Event) {
	return func(event eh.Event) {
		URI := uriGetter(event)

		ch <- persistMeta{
			URI,
			jsonAction,
			saveDir,
			bkupDir,
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
		switch metaData.actionType {
		case SAVE:
			updateJSON(logger, bus, URI, metaData.fileLoc)
		case DELETE:
			deleteAndRestoreJSON(logger, bus.GetBus(), URI, metaData.fileLoc, metaData.bkupLoc)
		default:
			logger.Warn("Unknown Action", "Action", metaData.actionType, "URI", URI)
		}
	}
}

func getJSONFilePath(uri string, basePath string) string {
	urlSplit := strings.Split(uri, "/")
	mrdName := urlSplit[len(urlSplit)-1]
	return basePath + mrdName + ".json"
}

func deleteAndRestoreJSON(logger log.Logger, bus eh.EventBus, uri string, mrdPerm string, mrdRO string) {
	permFilePath := getJSONFilePath(uri, mrdRO)
	fileToDelete := getJSONFilePath(uri, mrdPerm)

	os.Remove(fileToDelete)
	if fileExists(permFilePath) {
		jsonstr, err := ioutil.ReadFile(permFilePath)
		if err != nil {
			logger.Warn("deleteAndRestoreJSON", "can not read json", permFilePath)
			return
		}

		eventData, err := eh.CreateEventData(tele.AddMRDCommandEvent)
		if err != nil {
			logger.Warn("deleteAndRestoreJSON", "can not create event AddMRDCommandEvent")
			return
		}

		err = json.Unmarshal(jsonstr, eventData)
		if err != nil {
			logger.Warn("deleteAndRestoreJSON", "can not unmarshal json to eventData")
			return
		}

		err = bus.PublishEvent(context.Background(), eh.NewEvent(tele.AddMRDCommandEvent, eventData, time.Now()))
		if err != nil {
			logger.Warn("deleteAndRestoreJSON", "restore did not succeed for", uri)
		}
	}
}

func updateJSON(logger log.Logger, bus busIntf, uri string, mrdPerm string) {
	// ensure that this dies eventually
	cmd, evt, err := metric.CommandFactory(tele.GenericGETCommandEvent)()
	if err != nil {
		return
	}

	cmd.(*tele.GenericGETCommandData).URI = uri
	filePath := getJSONFilePath(uri, mrdPerm)

	outputFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
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

func fileExists(fn string) bool {
	fd, err := os.Stat(fn)
	if os.IsNotExist(err) {
		return false
	}
	return !fd.IsDir()
}
