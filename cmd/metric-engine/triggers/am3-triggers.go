package triggers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"
	"golang.org/x/xerrors"

	"github.com/superchalupa/sailfish/cmd/metric-engine/fifocompat"
	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"

	log "github.com/superchalupa/sailfish/src/log"
)

const (
	triggerSubscribeEvent   eh.EventType = "Subscribe for Triggers"
	triggerUnsubscribeEvent eh.EventType = "Unsubscribe Triggers"
)

type busComponents interface {
	GetBus() eh.EventBus
}

type eventHandlingService interface {
	AddEventHandler(string, eh.EventType, func(eh.Event))
}

type SubscriberMap map[string]*os.File

type subscriptionData struct {
	namedPipeName string
}

// StartupTriggerProcessing will attach event handlers to handle subscriber events
func StartupTriggerProcessing(logger log.Logger, cfg *viper.Viper, am3Svc eventHandlingService, d busComponents) error {
	// setup programatic defaults. can be overridden in config file
	cfg.SetDefault("triggers.clientIPCPipe", "/var/run/telemetryservice/telsubandlclnotifypipe")
	cfg.SetDefault("triggers.subList", "/var/run/telemetryservice/telsvc_subscriptioninfo.json")

	// fetch the active subscriber list on startup
	subscriberListFile := cfg.GetString("triggers.subList")
	activeSubs, err := readSubFile(logger, subscriberListFile)
	if err != nil {
		logger.Crit("Error while restoring subscriber list.", "err", err)
	}

	// set up the event handler that will send triggers on the report on report generated events.
	am3Svc.AddEventHandler("Metric Report Generated", metric.ReportGenerated,
		MakeHandlerReportGenerated(logger, activeSubs, d.GetBus()))

	// set up the event handler to process subscription request.
	am3Svc.AddEventHandler("Subscribe for Triggers", triggerSubscribeEvent,
		MakeHandlerSubscribe(logger, activeSubs, subscriberListFile, d.GetBus()))

	// set up the event handler to process unsubscription request.
	am3Svc.AddEventHandler("Unsubscribe Triggers", triggerUnsubscribeEvent,
		MakeHandlerUnsubscribe(logger, activeSubs, subscriberListFile, d.GetBus()))

	// handle Subscription and LCL event related notifications
	go handleSubscriptionsAndLCLNotify(logger, cfg.GetString("triggers.clientIPCPipe"), d.GetBus())

	return nil
}

// only need to read file once on startup
func readSubFile(logger log.Logger, subFilePath string) (SubscriberMap, error) {
	logger.Crit("Reading saved subscriber map")
	activeSubs := SubscriberMap{}

	jsonstring, err := ioutil.ReadFile(subFilePath)
	if err != nil {
		return activeSubs, xerrors.Errorf("didn't read active telemetry subscriptions: %w", err)
	}

	sublist := []string{}
	err = json.Unmarshal(jsonstring, &sublist)
	if err != nil {
		return activeSubs, xerrors.Errorf("subscription unmarshal failed: %w", err)
	}

	for _, filename := range sublist {
		fd, err := os.OpenFile(filename, os.O_RDWR, os.ModeNamedPipe)
		if err != nil {
			logger.Crit("error opening subscriber pipe", "filename", filename, "err", err)
		}
		activeSubs[filename] = fd
	}

	return activeSubs, nil
}

func writeSubFile(logger log.Logger, activeSubs SubscriberMap, subFilePath string) {
	//re-write the persistent subscriber savefile (for restart of metric-engine)
	cfgSaveFd, err := os.Create(subFilePath)
	if err != nil {
		logger.Crit("Error creating the subscriber map file", "err", err)
		return
	}
	defer cfgSaveFd.Close()

	subs := []string{}
	for k := range activeSubs {
		subs = append(subs, k)
	}
	enc := json.NewEncoder(cfgSaveFd)
	enc.Encode(subs)
}

func closeActiveSubs(logger log.Logger, activeSubs SubscriberMap) {
	for k, subscriber := range activeSubs {
		err := subscriber.Close()
		if err != nil {
			logger.Warn("clean up pipe close failed", "err", err, "filepath", k)
		}
		delete(activeSubs, k)
	}
}

func MakeHandlerReportGenerated(logger log.Logger, subscriberMap SubscriberMap, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		notify, ok := event.Data().(*metric.ReportGeneratedData)
		if !ok {
			logger.Crit("Trigger report generated handler got an invalid data event", "event",
				event, "eventdata", event.Data())
			return
		}

		//send triggers to active subscribers
		for k, subscriber := range subscriberMap {
			fmt.Printf("Report due trigger to subscriber %s - %s \n", k, notify.Name)
			_, err := fmt.Fprintf(subscriber, "|%s|", strings.ToLower(notify.Name))
			if err != nil {
				// remove subscriber on error
				logger.Warn("Trigger notification to subscriber failed", "err", err)
				err := subscriber.Close()
				if err != nil {
					logger.Crit("error closing pipe on error", "err", err)
				}
				delete(subscriberMap, k)
			}
		}
	}
}

func MakeHandlerSubscribe(logger log.Logger, activeSubs SubscriberMap, subFilePath string, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		notify, ok := event.Data().(*subscriptionData)
		if !ok {
			logger.Crit("Subscription request handler got an invalid data event", "event", event, "eventdata", event.Data())
			return
		}
		subscribedPipe := strings.ToLower(notify.namedPipeName)

		fd, err := os.OpenFile(subscribedPipe, os.O_RDWR, os.ModeNamedPipe)
		if err != nil {
			logger.Crit("Error client subscription named pipe", "err", err)
			return
		}

		// Add our new subscriber to the list
		activeSubs[subscribedPipe] = fd

		// persist after changing
		writeSubFile(logger, activeSubs, subFilePath)
	}
}

func MakeHandlerUnsubscribe(logger log.Logger, activeSubs SubscriberMap, subFilePath string, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		notify, ok := event.Data().(*subscriptionData)
		if !ok {
			logger.Crit("Unsubscribe request handler got an invalid data event", "event",
				event, "eventdata", event.Data())
			return
		}

		//Remove the entry from subscriber map file which is maintained for persistency of
		//subscription across internal process restarts.
		unsubscribedPipe := strings.ToLower(notify.namedPipeName)
		fd, ok := activeSubs[unsubscribedPipe]
		if ok {
			fd.Close()
		}
		delete(activeSubs, unsubscribedPipe)

		// persist after changing
		writeSubFile(logger, activeSubs, subFilePath)
	}
}

// publishHelper will log/eat the error from PublishEvent since we can't do anything useful with it
func publishHelper(logger log.Logger, bus eh.EventBus, et eh.EventType, data eh.EventData) {
	err := bus.PublishEvent(context.Background(), eh.NewEvent(et, data, time.Now()))
	if err != nil {
		logger.Crit("Error publishing event. This should never happen!", "err", err)
	}
}

func processSubscriptionsOrLCLNotify(logger log.Logger, bus eh.EventBus, scanText string) {
	switch {
	case strings.HasPrefix(scanText, "subscribe@"):
		subFileName := scanText[len("subscribe@"):]
		fmt.Printf("Subscription request %s - %s \n", scanText, subFileName)
		publishHelper(logger, bus, triggerSubscribeEvent, &subscriptionData{namedPipeName: subFileName})

	case strings.HasPrefix(scanText, "unsubscribe@"):
		subFileName := scanText[len("unsubscribe@"):]
		fmt.Printf("Unsubscription request %s - %s \n", scanText, subFileName)
		publishHelper(logger, bus, triggerUnsubscribeEvent, &subscriptionData{namedPipeName: subFileName})

	default:
		for _, name := range strings.Split(scanText, ",") {
			evt, err := metric.NewRequestReportCommand(name)
			if err != nil {
				logger.Warn("Error creating report request command", "err", err)
				continue
			}
			publishHelper(logger, bus, evt.EventType(), evt.Data())
		}
	}
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
func handleSubscriptionsAndLCLNotify(logger log.Logger, pipePath string, bus eh.EventBus) {
	//Start listening on the client subscription named pipe IPC for
	//subscribe/unsubscribe and LCL event notifications
	err := fifocompat.MakeFifo(pipePath, 0660)
	if err != nil && !os.IsExist(err) {
		logger.Warn("Error creating Trigger (un)sub and LCL events IPC pipe", "err", err)
	}

	file, err := os.OpenFile(pipePath, os.O_CREATE, os.ModeNamedPipe)
	if err != nil {
		logger.Crit("Error opening Trigger (un)sub and LCL events IPC pipe", "err", err)
	}
	defer file.Close()

	nullWriter, err := os.OpenFile(pipePath, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		logger.Crit("Error opening Trigger (un)sub and LCL events IPC  pipe for (placeholder) write", "err", err)
	}
	// defer .Close() to keep linters happy. Inside we know we never exit...
	defer nullWriter.Close()

	s := bufio.NewScanner(file)
	s.Split(bufio.ScanWords)
	for s.Scan() {
		scanText := s.Text()
		processSubscriptionsOrLCLNotify(logger, bus, scanText)
	}

	panic("subscription cmd pipe closed. should never happen.")
}
