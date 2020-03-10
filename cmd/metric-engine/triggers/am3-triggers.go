package triggers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/cmd/metric-engine/fifocompat"
	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"

	log "github.com/superchalupa/sailfish/src/log"
)

const (
	triggerSubscribeEvent   eh.EventType = "Subscribe for Triggers"
	triggerUnsubscribeEvent eh.EventType = "Unsubscribe Triggers"
	printSubscriberMaps     eh.EventType = "Print Subscriber Maps"
)

type busComponents interface {
	GetBus() eh.EventBus
}

type eventHandlingService interface {
	AddEventHandler(string, eh.EventType, func(eh.Event))
}

// StartupTriggerProcessing will attach event handlers to handle subscriber events
func StartupTriggerProcessing(logger log.Logger, cfg *viper.Viper, am3Svc eventHandlingService,
	d busComponents) error {
	// setup programatic defaults. can be overridden in config file
	cfg.SetDefault("triggers.clientIPCPipe", "/var/run/telemetryservice/telsubandlclnotifypipe")
	cfg.SetDefault("triggers.subList", "/var/run/telemetryservice/telsvc_subscriptioninfo.json")

	activeSubs := make(map[string]*os.File)

	//Setup event handlers for
	//  - Metric reprot generated event
	//  - New subscription request event
	//  - New unsubscription request event
	setupEventHandlers(logger, d.GetBus(), am3Svc, activeSubs, cfg.GetString("triggers.subList"))

	// handle Subscription and LCL event related notifications
	go handleSubscriptionsAndLCLNotify(logger, cfg.GetString("triggers.clientIPCPipe"),
		cfg.GetString("triggers.subList"), activeSubs, d.GetBus())

	return nil
}

// Read the subscriber cache file to form the active subscriber list
func getSubscribers(logger log.Logger, subFilePath string, activeSubs map[string]*os.File) {
	subscribers, err := ioutil.ReadFile(subFilePath)
	if err != nil {
		logger.Info("There are no active telemetry subscriptions : " + err.Error())
		return
	}

	var subMap map[string]string
	jsonErr := json.Unmarshal(subscribers, &subMap)
	if jsonErr != nil {
		// Unmarshal failed. graceful return to accommodate future subscriptions
		logger.Warn("Subscription file unmarshal failed ", "err", jsonErr)
		return
	}

	for k, pipePath := range subMap {
		fmt.Printf("Identified subscriber %s - %s \n", k, pipePath)
		subWriter, err := os.OpenFile(pipePath, os.O_RDWR, os.ModeNamedPipe)
		if err != nil {
			//We gracefully continue and process other subscriptions
			logger.Warn("Error client subscription named pipe", "err", err)
			continue
		}
		activeSubs[k] = subWriter
	}

	fmt.Printf("active subscriber map size  - %d \n", len(activeSubs))
}

// Close all active subscription - executed in defered mode
func closeActiveSubs(logger log.Logger, activeSubs map[string]*os.File) {
	for k, openFile := range activeSubs {
		err := openFile.Close()
		if err != nil {
			logger.Warn("Clean-up - pipe close failed - ", "err", err)
		}
		delete(activeSubs, k)
	}
}

// Report generated am3 service notification handler
func MakeHandlerReportGenerated(logger log.Logger, subscriberMap map[string]*os.File,
	bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		notify, ok := event.Data().(*metric.ReportGeneratedData)
		if !ok {
			logger.Crit("Trigger report generated handler got an invalid data event", "event",
				event, "eventdata", event.Data())
			return
		}

		//send triggers to active subscribers
		reportName := strings.ToLower(notify.Name)
		for k, subWriter := range subscriberMap {
			fmt.Printf("Report due trigger to subscriber %s - %s \n", k, reportName)
			nBytes, writeErr := subWriter.WriteString(fmt.Sprintf("|%s|", reportName))
			if writeErr != nil {
				logger.Warn("Trigger notification to subscriber failed - ", "err", writeErr, "Num bytes ", nBytes)
			}
		}
	}
}

// Generate a random integeter with in the min max range
func randomInt(min, max int) int {
	return min + rand.Intn(max-min)
}

// Generate  a random alpha string of length len
func randomString(len int) string {
	randomLowRange := 65
	randomHighRange := 90
	bytes := make([]byte, len)
	for i := 0; i < len; i++ {
		bytes[i] = byte(randomInt(randomLowRange, randomHighRange))
	}
	return string(bytes)
}

// Subscribe request am3 service notification handler
func MakeHandlerSubscribe(logger log.Logger, activeSubs map[string]*os.File,
	subFilePath string, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		keyRandomIDLen := 9
		notify, ok := event.Data().(*subscriptionData)
		if !ok {
			logger.Crit("Subscription request handler got an invalid data event", "event",
				event, "eventdata", event.Data())
			return
		}
		//Update the subscriber map file with new subscriber,
		//which is maintained for persistency of
		//subscription across internal process restarts.
		subscribedPipe := strings.ToLower(notify.namedPipeName)
		key := fmt.Sprintf("subscriber-%s", randomString(keyRandomIDLen))

		var subMap map[string]string

		subscribers, err := ioutil.ReadFile(subFilePath)
		if err != nil {
			logger.Info("There are no active telemetry subscriptions : " + err.Error())
		} else {
			jsonErr := json.Unmarshal(subscribers, &subMap)
			if jsonErr != nil {
				// Unmarshal failed. graceful return to accommodate future subscriptions
				logger.Warn("Subscription file unmarshal failed ", "err", jsonErr)
				subMap = make(map[string]string)
			}
		}

		subMap[key] = subscribedPipe
		jsonSubscriberMap, _ := json.Marshal(subMap)
		subFile, err := os.Create(subFilePath)
		if err != nil {
			logger.Crit("Error creating the subscriber map file : " + err.Error())
			return
		}
		nBytes, writeErr := subFile.Write(jsonSubscriberMap)
		if writeErr != nil {
			logger.Warn("Subscription file update failed ", "err", writeErr, "Num bytes ", nBytes)
		}
		subFile.Close()

		//Update the active subscriber map
		subWriter, err := os.OpenFile(subscribedPipe, os.O_RDWR, os.ModeNamedPipe)
		if err != nil {
			logger.Crit("Error client subscription named pipe", "err", err)
		} else {
			activeSubs[key] = subWriter

			//The file handles are closed at exit
		}
	}
}

// Unsubscribe request am3 service notification handler
func MakeHandlerUnsubscribe(logger log.Logger, activeSubs map[string]*os.File,
	subFilePath string, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		notify, ok := event.Data().(*subscriptionData)
		if !ok {
			logger.Crit("Unsubscribe request handler got an invalid data event", "event",
				event, "eventdata", event.Data())
			return
		}

		subscribers, err := ioutil.ReadFile(subFilePath)
		if err != nil {
			logger.Crit("Subscriber map file does not exist : " + err.Error())
			return
		}

		//Remove the entry from subscriber map file which is maintained for persistency of
		//subscription across internal process restarts.
		unsubscribedPipe := strings.ToLower(notify.namedPipeName)
		var subMap map[string]string
		jsonErr := json.Unmarshal(subscribers, &subMap)
		if jsonErr != nil {
			logger.Crit("Subscription file unmarshal failed ", "err", jsonErr)
			return
		}

		for k, subNamedPipe := range subMap {
			if subNamedPipe == unsubscribedPipe {
				delete(subMap, k)
				//Update the active subscriber map
				err := activeSubs[k].Close()
				if err != nil {
					logger.Warn("Unsubscribed pipe close failed - ", "err", err)
				}
				delete(activeSubs, k)
				break
			}
		}

		jsonSubscriberMap, _ := json.Marshal(subMap)
		subFile, err := os.Create(subFilePath)
		if err != nil {
			logger.Crit("Error creating the subscriber map file : " + err.Error())
		} else {
			nBytes, writeErr := subFile.Write(jsonSubscriberMap)
			if writeErr != nil {
				logger.Crit("Subscription file write failed ", "err", writeErr, "Num bytes ", nBytes)
			}
		}

		subFile.Close()
	}
}

// debugging entry point for subscription interface
func MakeHandlerPrintSubscribers(logger log.Logger, activeSubs map[string]*os.File,
	subFilePath string, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		//Print subscription cache
		subscribers, err := ioutil.ReadFile(subFilePath)
		if err != nil {
			logger.Crit("Subscriber map file does not exist : " + err.Error())
			return
		}

		var subMap map[string]string
		jsonErr := json.Unmarshal(subscribers, &subMap)
		if jsonErr != nil {
			logger.Crit("Subscription file unmarshal failed ", "err", jsonErr)
			return
		}

		for k, subNamedPipe := range subMap {
			fmt.Printf("subscriber in cache %s - %s \n", k, subNamedPipe)
		}

		//Print active subscriptions
		fmt.Printf("active subscriber map size  - %d \n", len(activeSubs))
		for k, fileHandle := range activeSubs {
			fmt.Printf("active subscriber %s - %p \n", k, fileHandle)
		}
	}
}

// publishHelper will log/eat the error from PublishEvent since we can't do anything useful with it
func publishHelper(logger log.Logger, bus eh.EventBus, event eh.Event) {
	err := bus.PublishEvent(context.Background(), event)
	if err != nil {
		logger.Crit("Error publishing event. This should never happen!", "err", err)
	}
}

type subscriptionData struct {
	namedPipeName string
}

func setupEventHandlers(logger log.Logger, bus eh.EventBus, am3Svc eventHandlingService,
	activeSubs map[string]*os.File, subscriberListFile string) {
	// set up the event handler that will send triggers on the report on report generated events.
	am3Svc.AddEventHandler("Metric Report Generated", metric.ReportGenerated,
		MakeHandlerReportGenerated(logger, activeSubs, bus))

	// set up the event handler to process subscription request.
	am3Svc.AddEventHandler("Subscribe for Triggers", triggerSubscribeEvent,
		MakeHandlerSubscribe(logger, activeSubs, subscriberListFile, bus))

	// set up the event handler to process unsubscription request.
	am3Svc.AddEventHandler("Unsubscribe Triggers", triggerUnsubscribeEvent,
		MakeHandlerUnsubscribe(logger, activeSubs, subscriberListFile, bus))

	// set up the event handler to print current subscriptions.
	am3Svc.AddEventHandler("Print Subscriber Maps", printSubscriberMaps,
		MakeHandlerPrintSubscribers(logger, activeSubs, subscriberListFile, bus))
}

func processSubscriptionsOrLCLNotify(logger log.Logger, bus eh.EventBus, scanText string) {
	if strings.HasPrefix(scanText, "subscribe@") {
		fmt.Printf("Report subscription request %s - %s \n", scanText, scanText[10:])
		publishHelper(logger, bus, eh.NewEvent(triggerSubscribeEvent,
			&subscriptionData{namedPipeName: scanText[10:]}, time.Now()))
		return
	}

	if strings.HasPrefix(scanText, "unsubscribe@") {
		fmt.Printf("Report unsubscription request %s - %s \n", scanText, scanText[12:])
		publishHelper(logger, bus, eh.NewEvent(triggerUnsubscribeEvent,
			&subscriptionData{namedPipeName: scanText[12:]}, time.Now()))
		return
	}

	if strings.HasPrefix(scanText, "printInternalMaps") {
		publishHelper(logger, bus, eh.NewEvent(printSubscriberMaps, nil, time.Now()))
		return
	}

	//LCL events
	reports := strings.Split(scanText, ",")
	for i := range reports {
		publishHelper(logger, bus, eh.NewEvent(metric.RequestReport,
			&metric.RequestReportData{Name: reports[i]}, time.Now()))
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
func handleSubscriptionsAndLCLNotify(logger log.Logger, pipePath string, subscriberListFile string,
	activeSubs map[string]*os.File, bus eh.EventBus) {
	// fetch the active subscriber list on startup
	getSubscribers(logger, subscriberListFile, activeSubs)
	// closing active subscription handles - to keep linters happy. Inside we know we never exit...
	defer closeActiveSubs(logger, activeSubs)

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
		scanText := strings.ReplaceAll(s.Text(), " ", "")
		fmt.Printf("New (Un)Subscrition request/LCL event message - %s\n", scanText)
		processSubscriptionsOrLCLNotify(logger, bus, scanText)
	}
}
