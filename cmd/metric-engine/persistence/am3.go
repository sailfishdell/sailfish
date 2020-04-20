package persistence

import (
	"context"
	"encoding/json"
	"fmt"
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
	eventL "github.com/superchalupa/sailfish/src/looplab/event"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
)

type eventHandler interface {
	AddEventHandler(string, eh.EventType, func(eh.Event)) error
}

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

func PersistenceHandler(ctx context.Context, logger log.Logger, cfg *viper.Viper, am3Svc eventHandler, d busIntf) func() {
	pTopDir := cfg.GetString("main.persistencetopdir")
	pROTopDir := cfg.GetString("main.persistenceROtopdir")

	for _, i := range []struct {
		am3name   string
		eventType eh.EventType
		savedir   string
	}{
		{"Create MRD", tele.AddMRDResponseEvent, pTopDir},
		{"Update MRD", tele.UpdateMRDResponseEvent, pTopDir},
	} {
		am3Svc.AddEventHandler(i.am3name, i.eventType, MakeHandlerUpdateJSON(logger, d, i.savedir))
	}

	for _, i := range []struct {
		am3name   string
		eventType eh.EventType
		savedir   string
		rodir     string
	}{
		{"Delete MRD", tele.DeleteMRDResponseEvent, pTopDir, pROTopDir},
	} {
		am3Svc.AddEventHandler(i.am3name, i.eventType, MakeHandlerDeleteJSON(logger, d, i.rodir, i.savedir))

	}
	return nil
}

// I can have setup function to add.. persistence
func MakeHandlerDeleteJSON(logger log.Logger, bus busIntf, mrdRO string, mrdperm string) func(eh.Event) {

	return func(event eh.Event) {
		sj, ok := event.Data().(urigetter)
		if !ok {
			logger.Warn("MakeHandlerDeleteJSON", "missing", "getURI")
			return
		}
		mrdRedfishPath := sj.GetURI()

		urlSplit := strings.Split(mrdRedfishPath, "/")
		mrdName := urlSplit[len(urlSplit)-1]

		permFilePath := mrdRO + mrdName + ".json"
		fileToDelete := mrdperm + mrdName + ".json"

		os.Remove(fileToDelete)
		// TODO  add a common place to read JSON and send event
		if fileExists(permFilePath) {
			jsonstr, err := ioutil.ReadFile(permFilePath)
			if err != nil {
				logger.Warn("MakeHandlerDeleteJSON", "can not read json", permFilePath)
				return
			}

			eventData, err := eh.CreateEventData(tele.AddMRDCommandEvent)
			if err != nil {
				logger.Warn("MakeHandlerDeleteJSON", "can not create event AddMRDCommandEvent")
				return
			}

			err = json.Unmarshal(jsonstr, eventData)
			if err != nil {
				logger.Warn("MakeHandlerDeleteJSON", "can not unmarshal json to eventData")
				return
			}

			evt := eventL.NewSyncEvent(tele.AddMRDCommandEvent, eventData, time.Now())
			evt.Add(1)
			err = bus.GetBus().PublishEvent(context.Background(), evt)
		}
	}
}

func MakeHandlerUpdateJSON(logger log.Logger, bus busIntf, mrdPerm string) func(eh.Event) {
	return func(event eh.Event) {

		// ensure that this dies eventually
		cmd, evt, err := metric.CommandFactory(tele.GenericGETCommandEvent)()
		if err != nil {
			return
		}

		// update to single liner
		sj, ok := event.Data().(urigetter)
		if !ok || sj.GetURI() == "" {
			logger.Warn("MakeHandlerUpdateJSON", "missing", "getURI")
			return
		}

		mrdRedfishPath := sj.GetURI()
		cmd.(*tele.GenericGETCommandData).URI = mrdRedfishPath

		urlSplit := strings.Split(mrdRedfishPath, "/")
		mrdName := urlSplit[len(urlSplit)-1]

		mrdPath := mrdPerm + "/" + mrdName + ".json"

		outputFile, err := os.OpenFile(mrdPath, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			fmt.Printf("ERROR: %s\n", err.Error())
			return
		}
		cmd.SetResponseHandlers(nil, nil, outputFile)

		go func() {
			const maxSaveWaitTime = 2 * time.Second
			ctx, cancel := context.WithTimeout(context.Background(), maxSaveWaitTime)
			defer cancel()
			l := eventwaiter.NewListener(ctx, logger, bus.GetWaiter(), cmd.ResponseWaitFn())

			l.Name = "JSON Persistence"
			err = bus.GetBus().PublishEvent(ctx, evt)
			if err != nil {
				fmt.Println("ERROR! %s", err.Error())
			}

			defer outputFile.Close()
			defer l.Close()
			l.Wait(ctx)
			fmt.Println("Exit Update JSON - success")
		}()
	}
}

func fileExists(fn string) bool {
	fd, err := os.Stat(fn)
	if os.IsNotExist(err) {
		return false
	}
	return !fd.IsDir()
}
