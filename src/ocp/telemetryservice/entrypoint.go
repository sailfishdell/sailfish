package telemetryservice

import (
	"context"
	"fmt"
	"strings"
	//"sync"
	"time"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

var (
	tele_md = domain.PluginType("certinfo")
)

//const (
//	telemetry_MD = eh.EventType("MD")
//	telemetry_MRD = eh.EventType("MRD")
//)
//
//
//func init() {
//	eh.RegisterEventData(telemetry_MD, func() eh.EventData { return &MetricDefinition })
//}
//
//
//type telemetry_MD struct {
//	Description string
//	EntryType   string
//	Id          int
//	Created     string
//	Message     string
//	MessageArgs []string
//	MessageID   string
//	Name        string
//	Severity    string
//	Category    string
//	Action      string
//	FQDD        string
//	LogAlert    string
//	EventId     string
//}


type syncEvent interface {
	Done()
}

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
}

type ReportCtx struct {
	// aggregate id.
}

type TelemetryService struct {
	d             *domain.DomainObjects
	ew            waiter
	ch            eh.CommandHandler
	logger        log.Logger
		nb 	*TelemetryReference	
}




// uncomment when feature is implemented and patchable.
type mrdPatch struct {
	mrdType    string
	mrdEnabled bool
	//	mrdHeart string
	//	suppressRepeat bool
	//	wildCard map[string][]string
}


func updateMDConfig (){
}

func New(ctx context.Context, logger log.Logger, chdler eh.CommandHandler, d *domain.DomainObjects) *TelemetryService {
	logger = logger.New("module", "telemetry")
	EventPublisher := eventpublisher.NewEventPublisher()
	d.EventBus.AddHandler(eh.MatchAnyEventOf(MetricValueEvent, domain.RedfishResourceRemoved, domain.RedfishResourcePropertiesUpdated2 ), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Telemetry Service"), eventwaiter.NoAutoRun)
	EventPublisher.AddObserver(EventWaiter)
	go EventWaiter.Run()

	ret := &TelemetryService{
		d:             d,
		ew:            EventWaiter,
		ch:            chdler,

		nb: 		initTelemetryNotebook(ctx, d),
		//mrdConfigL:    []mrdConfig{},
		//metric2Report: map[string][]*mrdConfig{}, // MetricProperties : []&mrdConfig, ex: System.Chassis.1/Thermal/Fan.Slot.1#Reading: []&mrdConfig
		logger:        logger,
	}

	ret.StartTelemetryService(ctx)
	return ret
}

//func (ts *TelemetryService) setMRDConfig(Id string, mrUUID eh.UUID, mrdUUID eh.UUID, mrURI string, mrdURI string, mrdEnabled bool, mrdType string, PropL []string) {
//	//var mrdP *mrdConfig = nil
//	for i := 0; i < len(ts.mrdConfigL); i++ {
//		if ts.mrdConfigL[i].name == Id {
//			mrdP = &ts.mrdConfigL[i]
//			break
//		}
//	}
//
//	if mrdP == nil {
//		ts.mrdConfigL = append(ts.mrdConfigL, mrdConfig{
//			name:    Id,
//			mrUUID:  mrUUID,
//			mrdUUID: mrdUUID,
//			mrURI:   mrURI,
//			mrdURI:  mrdURI,
//			config: mrdPatch{
//				mrdEnabled: mrdEnabled,
//				mrdType:    mrdType,
//			}})
//		mrdP = &ts.mrdConfigL[len(ts.mrdConfigL)-1]
//
//	} else {
//		ts.deleteMRDConfig(mrdP)
//
//		mrdP.name = Id
//		mrdP.mrdUUID = mrdUUID
//		mrdP.mrdURI = mrdURI
//		mrdP.config.mrdEnabled = mrdEnabled
//		mrdP.config.mrdType = mrdType
//		mrdP.mrUUID = mrUUID
//		mrdP.mrURI = mrURI
//	}
//
//	for i := 0; i < len(PropL); i++ {
//		pS := PropL[i]
//		_, ok := ts.metric2Report[pS]
//		if ok {
//			ts.metric2Report[pS] = append(ts.metric2Report[pS], mrdP)
//		} else {
//			ts.metric2Report[pS] = []*mrdConfig{mrdP}
//		}
//	}
//
//}

//func (ts *TelemetryService) deleteMRDConfig(mrdP *mrdConfig) {
//	for key, val := range ts.metric2Report {
//		for i := 0; i < len(val); i++ {
//			if val[i] == mrdP {
//				if len(val) == 1 {
//					val[i] = nil
//					val = nil
//				} else {
//					tmp := val[i]
//					val[i] = val[0]
//					val[0] = tmp
//					val[0] = nil
//
//					val = val[1:]
//				}
//
//			}
//
//			if len(val) == 0 {
//				delete(ts.metric2Report, key)
//			}
//		}
//
//	}
//}

// EC Sailfish sends MetricValueEvents northbound for consumption.  
// MSM Sailfish handles MetricValueEvents and create corresponding MR, and MR events
func (ts *TelemetryService) sendMetricEvent(ctx context.Context,  metricID string, metricValue interface{}, metricProp string ) {

	eventData := &MetricValueEventData{
		// metric id is related to the ID
		MetricId: metricID,
		MetricValue: metricValue,
		Timestamp:   time.Now().UTC().Format("2006-01-02T15:04:05-07:00"),
		MetricProperty:   metricProp,
	}
	ts.d.EventBus.PublishEvent(ctx, eh.NewEvent(MetricValueEvent, eventData, time.Now()))
}

func (ts *TelemetryService) StartTelemetryService(ctx context.Context) error {
	eh.RegisterCommand(func() eh.Command {
		return &POST{ts: ts, d: ts.d}
	})


	// listener will only return events that match requirements in NewListener
	listener := eventwaiter.NewListener(ctx, ts.logger, ts.d.GetWaiter(), func(ev eh.Event) bool {
	        switch typ:=ev.EventType(); typ {
		case AddedMRDEvent:
			return true
		case domain.RedfishResourcePropertiesUpdated2: 
	 		if _, ok := ev.Data().(*domain.RedfishResourcePropertiesUpdatedData2); ok {
				return true // need good URI validation here
	 		}
		case domain.RedfishResourceRemoved:
			if data, ok := ev.Data().(*domain.RedfishResourceRemovedData); ok {
				return strings.Contains(data.ResourceURI, "/redfish/v1/TelemetryService/MetricReportDefinitions/")
			}
		
		}
	 	return false
		
	})
	baseMRD := "/redfish/v1/TelemetryService/MetricReportDefinitions" 

	// how do I know when to close the listener?
	//defer listener.Close()

	// hmm but I could have.. this as a eventhandler.. nothing depends on the other... interesting
	go listener.ProcessEvents(ctx, func(event eh.Event) {
		switch typ := event.EventType(); typ {
		case AddedMRDEvent:
			fmt.Println("RECEIVED ADDED MRD %T", event.Data())
	 		if data, ok := event.Data().(*MRDData); ok {
				ts.nb.MRDConfigAdd( data)
			}

		case domain.RedfishResourcePropertiesUpdated2:
			if data, ok :=event.Data().(*domain.RedfishResourcePropertiesUpdatedData2); ok {
				if strings.HasPrefix(data.ResourceURI, baseMRD) {
					// update internal MRD metrics
					///teleConfig.updateMRDConfig(data.ID,nil, 0)
				} else {
					// send metrics that are part of MRD
					metrics:= ts.nb.getValidMetrics( data)
					fmt.Println("valid metrics", metrics)

					for metricid, PV:= range(metrics){
						for prop, value:= range(PV){
							ts.sendMetricEvent(ctx, metricid, value, prop)
						}
					}
				}
			}
		case domain.RedfishResourceRemoved:
			if data, ok := event.Data().(*domain.RedfishResourceRemovedData); ok {
				ts.nb.MRDConfigDelete(data.ResourceURI)
			}
		
		}
	})
			
	return nil
}


func (ts *TelemetryService) CreateMetricReportDefinition(ctx context.Context, mrd MRDData, data *domain.HTTPCmdProcessedData) (bool, eh.UUID ) {

	bad_request := domain.ExtendedInfo{
		Message:             "The service detected a malformed request body that it was unable to interpret.",
		MessageArgs:         []string{},
		MessageArgsCt:       0,
		MessageId:           "Base.1.0.UnrecognizedRequestBody",
		RelatedProperties:   []string{"Attributes"}, //FIX ME
		RelatedPropertiesCt: 1,                      //FIX ME
		Resolution:          "Correct the request body and resubmit the request if it failed.",
		Severity:            "Warning",
	}

	log.ContextLogger(ctx, "submit_mrd").Debug("got test metric report event", "event_data", mrd)
	errmmsg := ""
	errmFmt := "%s, "

	if mrd.Id == "" {
		errmmsg += fmt.Sprintf(errmFmt, "Id")
	}

	if len(mrd.Metrics) == 0 {
		errmmsg += fmt.Sprintf(errmFmt, "Metric")
	}

	if mrd.MetricReportDefinitionType == "" {
		errmmsg += fmt.Sprintf(errmFmt, "MetricReportDefinitionType")
	}
	if errmmsg != "" {
		domain.AddToEEMIList(data.Results.(map[string]interface{}), bad_request, false)
		fmt.Println(data.Results)
		data.StatusCode = 400
		return false, "" 
	}

	mrdURL := "/redfish/v1/TelemetryService/MetricReportDefinitions/" + mrd.Id
	
	// Check if MRD metric properties is valid 
	ok := ts.nb.CleanAndValidateMRD(&mrd )
	if !ok {
		domain.AddToEEMIList(data.Results.(map[string]interface{}), bad_request, false)
		fmt.Println(data.Results)
		data.StatusCode = 400
		return false, ""
	}

	metricSlice := []map[string]interface{}{}
	// transform metrics in map format
	for i:=0; i<len(mrd.Metrics); i++ {
		m:=map[string]interface{}{
			"Metricid": mrd.Metrics[i].MetricID, 
			"MetricProperties": mrd.Metrics[i].MetricProperties,
			"CollectionDuration": mrd.Metrics[i].CollectionDuration,
			"CollectionFunction": mrd.Metrics[i].CollectionFunction,
			"CollectionTimeScope":mrd.Metrics[i].CollectionTimeScope,
		}
		metricSlice= append(metricSlice, m) 
	}

	mrduuid := eh.NewUUID()
	ts.ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          mrduuid,
			ResourceURI: mrdURL,
			Type:        "#MetricReportDefinition.v1_1_2.MetricReportDefinition",
			Context:     "/redfish/v1/$metadata#MetricReportDefinition.MetricReportDefinition",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{"ConfigureManager"},
			},
			Properties: map[string]interface{}{
				"Id":                         mrd.Id,
				"Description":                mrd.Description,
				"Name":                       mrd.Name,
				"MetricReportDefinitionType": mrd.MetricReportDefinitionType,
				// todo "Schedule": mrdSchedule,
				"MetricReportDefinitionEnabled@meta": map[string]interface{}{
					"DEFAULT": mrd.MetricReportDefinitionEnabled,
					"PATCH": map[string]interface{}{
						"plugin": "GenericBool"}},
				"Metrics": metricSlice, 
				"SuppressRepeatedMetricValue@meta": map[string]interface{}{
					"DEFAULT": mrd.SuppressRepeatedMetricValue,
					"PATCH": map[string]interface{}{
						"plugin": "GenericBool"}},
				"MetricReportHeartbeatInterval": mrd.MetricReportHeartbeatInterval,
				"Wildcards":                     mrd.Wildcards,
			},
		})

	default_msg := domain.ExtendedInfo{}

	data.StatusCode = 201
	data.Results = default_msg.GetDefaultExtendedInfo()
	// send cleaned up MRD as a telemetry reference point
	ts.d.EventBus.PublishEvent(ctx, eh.NewEvent(AddedMRDEvent,&mrd , time.Now()))
	return true, mrduuid
}
