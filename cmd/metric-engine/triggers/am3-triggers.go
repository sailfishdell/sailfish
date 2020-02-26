package triggers

import (
	"bufio"
	"context"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"encoding/json"
	"time"
	"fmt"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"
	"github.com/superchalupa/sailfish/src/looplab/event"

	log "github.com/superchalupa/sailfish/src/log"
)

type busComponents interface {
	GetBus() eh.EventBus
}

type eventHandlingService interface {
	AddEventHandler(string, eh.EventType, func(eh.Event))
}

func getSubscribers (logger log.Logger, subFilePath string) (subscriberMap map[string]*os.File){
	subscribers, err := ioutil.ReadFile(subFilePath)
	if err != nil {
		panic("Error initializing base telemetry subsystem: " + err.Error())
	}
	
    activeSubs := make(map[string]*os.File)

	var subs interface{}
	json.Unmarshal(subscribers, &subs)
	subMap := subs.(map[string]interface{})

	for k, pipePath := range subMap {
		switch pipePath := pipePath.(type) {
			case string:
				fmt.Printf("Identified subscriber %s - %s \n", k, pipePath)				
				subWriter, err := os.OpenFile(pipePath, os.O_RDWR, os.ModeNamedPipe)
				if err != nil {
					logger.Crit("Error opening UDB pipe for (placeholder) write", "err", err)
				}
				activeSubs[k] = subWriter
			default:
				fmt.Printf("(unknown)")
		}
	}
	
	return activeSubs	
}

// StartupUDBImport will attach event handlers to handle import UDB import
func StartupTriggerProcessing(logger log.Logger, cfg *viper.Viper, am3Svc eventHandlingService, d busComponents) error {
	// setup programatic defaults. can be overridden in config file
	cfg.SetDefault("triggers.subAndLCLNotifyPipe", "/var/run/telemetryservice/telsubandlclnotifypipe")
	cfg.SetDefault("triggers.subList", "/var/run/telemetryservice/telsvc_subscriptioninfo.json")
	
	subMap := getSubscribers(logger, cfg.GetString("triggers.subList"))

	bus := d.GetBus()
	// set up the event handler that will send triggers on the report on report generated events.
	am3Svc.AddEventHandler("Metric Report Generated", metric.ReportGenerated, 
							MakeHandlerReportGenerated(logger, subMap, bus))

	// handle UDB notifications
	go handleSubscriptionsAndLCLNotifyPipe(logger, cfg.GetString("triggers.subAndLCLNotifyPipe"), d)

	//panic("Error initializing telemetry trigger subsystem")

	return nil
}

func MakeHandlerReportGenerated(logger log.Logger, subscriberMap map[string]*os.File, 
											bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		notify, ok := event.Data().(*ReportGeneratedData)
		if !ok {
			fmt.Printf("***** Trigger - %s \n", event.Data())
			logger.Crit("UDB Change Notifier message handler got an invalid data event", "event", 
				event, "eventdata", event.Data())
			return
		}
		
		//send triggers
		for k, subWriter := range subscriberMap {
			fmt.Printf("***** Identified subscriber %s - %s \n", k, subWriter)
			fmt.Printf("***** Trigger - %s \n", strings.ToLower(notify.Name))			
		}
	}
}

// ReportGeneratedData is the event data structure emitted after reports are generated
type ReportGeneratedData struct {
	Name string
}


func publishAndWait(logger log.Logger, bus eh.EventBus, et eh.EventType, data eh.EventData) {
	evt := event.NewSyncEvent(et, data, time.Now())
	evt.Add(1)
	err := bus.PublishEvent(context.Background(), evt)
	if err != nil {
		logger.Crit("Error publishing event. This should never happen!", "err", err)
	}
	evt.Wait()
}

type RequestReportData struct {
	Name  string
}

func makeSplitFunc() (*RequestReportData, func([]byte, bool) (int, []byte, error)) {
	n := &RequestReportData{}
	split := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF {
			return 0, nil, io.EOF
		}

		if strings.Contains(string(data), "subscribe@") {
			//Store at triggers.subList and start sending report notifications 	
			return len(data), []byte("s"), nil
		}
		
		if strings.Contains(string(data), "unsubscribe@") {
			//Remove from triggers.subList and stop sending report notifications 	
			return len(data), []byte("u"), nil
		}
		
		//Process LCL event list by sending 'RequestReport' event to telemetry-db 	
		
		return len(data), nil, nil
	}
	return n, split
}

// handleSubscriptionsAndLCLNotifyPipe will handle the LCL notification events from 
// event manager and any other subscription requests
//
// Data format we get:
// reportname,<reportname_n> - for LCL events which need reprot triggers
// subscribe@<pipename> for subscription request on pipe
// unsubscribe@<pipename> for unsubscription request on pipe
//
// The reader of the named pipe gets an EOF when the last writer exits. To
// avoid this, we'll simply open it ourselves for writing and never close it.
// This will ensure the pipe stays around forever without eof... That's what
// nullWriter is for, below.
func handleSubscriptionsAndLCLNotifyPipe(logger log.Logger, pipePath string, d busComponents) {
	err := makeFifo(pipePath, 0660)
	if err != nil && !os.IsExist(err) {
		logger.Warn("Error creating UDB pipe", "err", err)
	}

	file, err := os.OpenFile(pipePath, os.O_CREATE, os.ModeNamedPipe)
	if err != nil {
		logger.Crit("Error opening UDB pipe", "err", err)
	}

	defer file.Close()

	nullWriter, err := os.OpenFile(pipePath, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		logger.Crit("Error opening UDB pipe for (placeholder) write", "err", err)
	}

	// defer .Close() to keep linters happy. Inside we know we never exit...
	defer nullWriter.Close()
	

	n, split := makeSplitFunc()
	s := bufio.NewScanner(file)
	s.Split(split)
	for s.Scan() {
		if s.Text() == "t" {
			publishAndWait(logger, d.GetBus(), metric.RequestReport, &metric.RequestReportData{Name: n.Name}) //TODO - pass the data
		}
	}
}
