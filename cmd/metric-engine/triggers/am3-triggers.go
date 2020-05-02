package triggers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"
	"golang.org/x/xerrors"

	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"
	"github.com/superchalupa/sailfish/src/fileutils"
	"github.com/superchalupa/sailfish/src/looplab/event"
	"github.com/superchalupa/sailfish/src/ocp/am3"

	log "github.com/superchalupa/sailfish/src/log"
)

const (
	triggerSubscribeEvent   eh.EventType = "Subscribe for Triggers"
	triggerUnsubscribeEvent eh.EventType = "Unsubscribe Triggers"
	printSubscriberMaps     eh.EventType = "Print Subscriber Maps"

	writeTimeout = 20 * time.Millisecond
)

type busComponents interface {
	GetBus() eh.EventBus
}

type SubscriberMap map[string]*os.File

type subscriptionData struct {
	namedPipeName string
}

// StartupTriggerProcessing will attach event handlers to handle subscriber events
func StartupTriggerProcessing(logger log.Logger, cfg *viper.Viper, am3Svc am3.Service,
	d busComponents) error {
	logger = log.With(logger, "module", "trigger") // set up logger module so that we can filter
	activeSubs := make(map[string]*os.File)

	// setup programatic defaults. can be overridden in config file
	cfg.SetDefault("triggers.clientIPCPipe", "/var/run/telemetryservice/telsubandlclnotifypipe")
	cfg.SetDefault("triggers.subList", "/var/run/telemetryservice/telsvc_subscriptioninfo.json")
	cfg.SetDefault("triggers.CPURegisterBINFile", "/flash/data0/IERR_LOG/IERR_LOG1/IERRMSRBIN.bin")

	// handle Subscription and LCL event related notifications
	handleSubscriptionsAndLCLNotify(logger, cfg.GetString("triggers.clientIPCPipe"),
		cfg.GetString("triggers.subList"), activeSubs, d.GetBus())

	//Setup event handlers for
	//  - Metric report generated event
	//  - New subscription request event
	//  - New unsubscription request event
	err := setupEventHandlers(logger, d.GetBus(), am3Svc, activeSubs, cfg)
	if err != nil {
		return err
	}

	return nil
}

// only need to read file once on startup
func readSubFile(logger log.Logger, subFilePath string, activeSubs SubscriberMap) error {
	logger.Info("Reading saved subscriber map")
	jsonstring, err := ioutil.ReadFile(subFilePath)
	if err != nil {
		return xerrors.Errorf("didn't read active telemetry subscriptions: %w", err)
	}

	sublist := []string{}
	err = json.Unmarshal(jsonstring, &sublist)
	if err != nil {
		return xerrors.Errorf("subscription unmarshal failed: %w", err)
	}

	for _, filename := range sublist {
		logger.Info("Restoring subscriber", "filename", filename)
		// have to use os.O_RDWR to get non-blocking behavior, otherwise this call hangs
		fd, err := os.OpenFile(filename, os.O_RDWR, 0o664)
		if err != nil {
			logger.Crit("Error opening saved subscriber fifo, skipping", "filename", filename, "err", err)
			continue
		}
		activeSubs[filename] = fd
	}
	return nil
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
	if enc != nil {
		err := enc.Encode(subs)
		if err != nil {
			logger.Crit("Error ecoding json content for subscription file", "err", err)
		}
	}
}

const invalidEventStr = "handler got unexpected type of data event, skipping"

// Report generated am3 service notification handler
func MakeHandlerReportGenerated(logger log.Logger, activeSubs map[string]*os.File,
	bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		notify, ok := event.Data().(*metric.ReportGeneratedData)
		if !ok {
			logger.Crit(invalidEventStr, "EventType", event.EventType(), "eventdata", event.Data())
			return
		}

		//send triggers to active subscribers
		for k, subscriber := range activeSubs {
			logger.Info("Report generated. Trigger subscribers.", "sub", k, "MRName", notify.MRName)
			err := subscriber.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err != nil {
				logger.Warn("Error setting write deadline for subscriber", "filename", k, "err", err)
			}
			_, writeErr := subscriber.Write([]byte("|" + notify.MRName + "|"))
			err = subscriber.SetWriteDeadline(time.Time{}) // zero - remove deadline
			if err != nil {
				logger.Warn("Error setting write deadline for subscriber", "filename", k, "err", err)
			}
			if writeErr != nil {
				// remove subscriber on error
				logger.Warn("Trigger notification to subscriber failed", "filename", k, "err", writeErr)
				delete(activeSubs, k)
				err := subscriber.Close()
				if err != nil {
					logger.Warn("error closing pipe on error", "filename", k, "err", err)
				}
			}
		}
	}
}

// Subscribe request am3 service notification handler
func MakeHandlerSubscribe(logger log.Logger, activeSubs map[string]*os.File,
	subFilePath string, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		notify, ok := event.Data().(*subscriptionData)
		if !ok {
			logger.Crit(invalidEventStr, "EventType", event.EventType(), "eventdata", event.Data())
			return
		}
		subscribedPipe := strings.ToLower(notify.namedPipeName)

		// have to use os.O_RDWR to get non-blocking behavior, otherwise this call can hang forever
		fd, err := os.OpenFile(subscribedPipe, os.O_RDWR, 0o660)
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

// Unsubscribe request am3 service notification handler
func MakeHandlerUnsubscribe(logger log.Logger, activeSubs map[string]*os.File,
	subFilePath string, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		notify, ok := event.Data().(*subscriptionData)
		if !ok {
			logger.Crit(invalidEventStr, "EventType", event.EventType(), "eventdata", event.Data())
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

// debugging entry point for subscription interface
func MakeHandlerPrintSubscribers(logger log.Logger, activeSubs map[string]*os.File,
	subFilePath string, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		//Print active subscriptions
		fmt.Printf("active subscriber map size  - %d \n", len(activeSubs))
		for k, fileHandle := range activeSubs {
			fmt.Printf("active subscriber %s - %p \n", k, fileHandle)
		}
	}
}

func setupEventHandlers(
	logger log.Logger,
	bus eh.EventBus,
	am3Svc am3.Service,
	activeSubs map[string]*os.File,
	cfg *viper.Viper, // don't pass this anywhere out of this func
) error {
	subscriberListFile := cfg.GetString("triggers.subList")
	// set up the event handler that will send triggers on the report on report generated events.
	err := am3Svc.AddEventHandler("Metric Report Generated", metric.ReportGenerated,
		MakeHandlerReportGenerated(logger, activeSubs, bus))
	if err != nil {
		return xerrors.Errorf("Failed to register event handler: %w", err)
	}

	// set up the event handler to process subscription request.
	err = am3Svc.AddEventHandler("Subscribe for Triggers", triggerSubscribeEvent,
		MakeHandlerSubscribe(logger, activeSubs, subscriberListFile, bus))
	if err != nil {
		return xerrors.Errorf("Failed to register event handler: %w", err)
	}

	// set up the event handler to process unsubscription request.
	err = am3Svc.AddEventHandler("Unsubscribe Triggers", triggerUnsubscribeEvent,
		MakeHandlerUnsubscribe(logger, activeSubs, subscriberListFile, bus))
	if err != nil {
		return xerrors.Errorf("Failed to register event handler: %w", err)
	}

	// set up the event handler to print current subscriptions.
	err = am3Svc.AddEventHandler("Print Subscriber Maps", printSubscriberMaps,
		MakeHandlerPrintSubscribers(logger, activeSubs, subscriberListFile, bus))
	if err != nil {
		return xerrors.Errorf("Failed to register event handler: %w", err)
	}

	//Subscription only for processing CPU registers
	cpuIERRFile := cfg.GetString("triggers.CPURegisterBINFile")
	err = am3Svc.AddEventHandler("Generate Metric Report", metric.GenerateReportCommandEvent, MakeHandlerSubscriberCPURegisters(logger, cpuIERRFile, bus))
	return nil
}

// Subscribe request am3 service notification handler
func MakeHandlerSubscriberCPURegisters(logger log.Logger, cpuIERRFile string, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		report, ok := event.Data().(*metric.GenerateReportCommandData)
		if !ok {
			logger.Crit("Trigger report generated handler got an invalid data event", "event",
				event, "eventdata", event.Data())
			return
		}
		if report.MRDName != "CPURegisters" {
			return
		}

		go CPURegisterFileHandling(logger, bus, cpuIERRFile)
	}
}

const sleepTimeCPUBin = 250 * time.Millisecond
const totalCPUBinTimeout = 60 * time.Second

func CPURegisterFileHandling(logger log.Logger, bus eh.EventBus, cpubinfile string) {
	i := int64(0)
	for {
		_, err := os.Stat(cpubinfile)
		if err == nil {
			break // yay, it exists
		}
		if i++; i > (int64(totalCPUBinTimeout) / int64(sleepTimeCPUBin)) {
			logger.Crit("WAITED LONG ENOUGH, EXITING")
			return
		}
		time.Sleep(sleepTimeCPUBin)
	}

	publishHelper(logger, bus, metric.ReportGenerated, &metric.ReportGeneratedData{MRDName: "CPURegisters", MRName: "CPURegisters"})
	return
}

// publishHelper will log/eat the error from PublishEvent since we can't do anything useful with it
func publishHelper(logger log.Logger, bus eh.EventBus, et eh.EventType, data eh.EventData) {
	evt := event.NewSyncEvent(et, data, time.Now())
	evt.Add(1)
	err := bus.PublishEvent(context.Background(), evt)
	if err != nil {
		logger.Crit("Error publishing event. This should never happen!", "err", err)
		return
	}
	evt.Wait()
}

const (
	subscribeStr      = "subscribe@"
	subscribeStrLen   = len(subscribeStr)
	unsubscribeStr    = "unsubscribe@"
	unsubscribeStrLen = len(unsubscribeStr)
)

func processPipeInput(logger log.Logger, bus eh.EventBus, scanText string) {
	logger.Debug("Process command pipe request", "scantext", scanText)
	switch {
	case strings.HasPrefix(scanText, subscribeStr):
		subFileName := scanText[subscribeStrLen:]
		logger.Info("Subscription request", "subFileName", subFileName)
		publishHelper(logger, bus, triggerSubscribeEvent, &subscriptionData{namedPipeName: subFileName})

	case strings.HasPrefix(scanText, unsubscribeStr):
		subFileName := scanText[unsubscribeStrLen:]
		logger.Info("Unsubscription request", "subFileName", subFileName)
		publishHelper(logger, bus, triggerUnsubscribeEvent, &subscriptionData{namedPipeName: subFileName})

	case strings.HasPrefix(scanText, "printInternalMaps"):
		logger.Info("Print internal maps")
		publishHelper(logger, bus, printSubscriberMaps, nil)

	default:
		reportDefList := strings.Split(scanText, ",")
		logger.Info("LCL triggered report gen", "reportDefList", reportDefList)
		for _, name := range reportDefList {
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
// we filter steam input to allow only alphanumeric and few special chars - /,@-
// / -> to include subscriber file path
// , -> to accommodate separator for trigger events
// @ -> subscribe/unsubscribe prefix - subscribe@/unsubscribe@
// - -> legacy redfish subsriber pipe path is /var/run/odatalite-providers/telemetryfifo

func handleSubscriptionsAndLCLNotify(logger log.Logger, pipePath string, subscriberListFile string,
	activeSubs map[string]*os.File, bus eh.EventBus) {
	// fetch the active subscriber list on startup
	readErr := readSubFile(logger, subscriberListFile, activeSubs)
	if readErr != nil {
		logger.Warn("Error restoring subscriber list, continuing with empty subscriber list", "err", readErr)
	}

	//Start listening on the client subscription named pipe IPC for
	//subscribe/unsubscribe and LCL event notifications
	go func() { // run forever in background
		logger.Info("Subscription handling goroutine started")
		reg := regexp.MustCompile("[^a-zA-Z0-9/,@-]+") // this will panic if it doesn't work, no need to nil check

		for {
			// clear out everything at startup and recreate files
			if !fileutils.IsFIFO(pipePath) {
				logger.Info("remove previous pipe path and recreate", "pipePath", pipePath)
				_ = os.Remove(pipePath)
				err := fileutils.MakeFifo(pipePath, 0660)
				if err != nil && !os.IsExist(err) {
					logger.Warn("Error creating Trigger (un)sub and LCL events IPC pipe", "err", err)
				}
			}

			logger.Info("Startup telemetry service trigger pipe processing.")
			// O_RDONLY opens with blocking behaviour, wont get past this line until somebody writes. that's ok.
			file, err := os.OpenFile(pipePath, os.O_RDONLY, 0o660) //golang octal prefix: 0o
			if err != nil {
				logger.Crit("Error opening Trigger (un)sub and LCL events IPC pipe", "err", err)
			}

			s := bufio.NewScanner(file)
			s.Split(bufio.ScanWords)
			for s.Scan() {
				scanText := reg.ReplaceAllString(s.Text(), "")
				processPipeInput(logger, bus, scanText)
			}

			logger.Info("EOF - telemetry service trigger pipe.")
			file.Close()
		}
	}()
}
