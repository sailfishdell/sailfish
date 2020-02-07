package telemetryservice

import (
	"context"
	"fmt"
	"strings"
	"time"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

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
	d      *domain.DomainObjects
	ew     waiter
	ch     eh.CommandHandler
	logger log.Logger
	nb     *NoteBook
}

// uncomment when feature is implemented and patchable.
type mrdPatch struct {
	mrdType    string
	mrdEnabled bool
	//	mrdHeart string
	//	suppressRepeat bool
	//	wildCard map[string][]string
}


func New(ctx context.Context, logger log.Logger, chdler eh.CommandHandler, d *domain.DomainObjects) *TelemetryService {
	logger = logger.New("module", "telemetry")
	EventPublisher := eventpublisher.NewEventPublisher()
	d.EventBus.AddHandler(eh.MatchAnyEventOf(AddedMRDEvent, MetricValueEvent, domain.RedfishResourceRemoved, domain.RedfishResourcePropertiesUpdated2), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Telemetry Service"), eventwaiter.NoAutoRun)
	EventPublisher.AddObserver(EventWaiter)
	go EventWaiter.Run()

	ret := &TelemetryService{
		d:  d,
		ew: EventWaiter,
		ch: chdler,

		nb: initTelemetryNotebook(ctx, d),
		logger: logger,
	}

	ret.StartTelemetryService(ctx)
	return ret
}

// EC Sailfish sends MetricValueEvents northbound for consumption into Metric Reports.
func (ts *TelemetryService) sendMetricEvent(ctx context.Context, metricID string, metricValue interface{}, metricProp string) {


	eventData := &MetricValueEventData{
		// metric id is related to the ID
		PropertyID:       metricID,
		Value:    metricValue,
		Timestamp:      time.Now().UTC().Format("2006-01-02T15:04:05-07:00"),
		Property: metricProp,
	}
	ts.d.EventBus.PublishEvent(ctx, eh.NewEvent(MetricValueEvent, eventData, time.Now()))
}

func (ts *TelemetryService) StartTelemetryService(ctx context.Context) error {
	eh.RegisterCommand(func() eh.Command {
		return &POST{ts: ts, d: ts.d}
	})

	// listener will only return events that match requirements in NewListener
	listener := eventwaiter.NewListener(ctx, ts.logger, ts.d.GetWaiter(), func(ev eh.Event) bool {
		switch typ := ev.EventType(); typ {
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

	// don't close listener defer listener.Close()

	// hmm but I could have.. this as a eventhandler.. nothing depends on the other... interesting
	go listener.ProcessEvents(ctx, func(event eh.Event) {
		switch typ := event.EventType(); typ {
		case AddedMRDEvent:
			if data, ok := event.Data().(*MRDData); ok {
				ts.nb.MRDConfigAdd(data)
			}

		case domain.RedfishResourcePropertiesUpdated2:
			if data, ok := event.Data().(*domain.RedfishResourcePropertiesUpdatedData2); ok {
				if strings.HasPrefix(data.ResourceURI, baseMRD) {
					// update internal MRD metrics
					///teleConfig.updateMRDConfig(data.ID,nil, 0)
				} else {
					// send metrics that are part of MRD
					metrics := ts.nb.getValidMetrics(data)

					for metricid, PV := range metrics {
						for prop, value := range PV {
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

func (ts *TelemetryService) CreateMetricReportDefinition(ctx context.Context, mrd MRDData, data *domain.HTTPCmdProcessedData) (bool, eh.UUID) {

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
		data.StatusCode = 400
		return false, ""
	}

	mrdURL := "/redfish/v1/TelemetryService/MetricReportDefinitions/" + mrd.Id

	// Check if MRD metric properties are related to a existing MD, or metric id
	ok := ts.nb.CleanAndValidateMRD(&mrd)
	if !ok {
		domain.AddToEEMIList(data.Results.(map[string]interface{}), bad_request, false)
		data.StatusCode = 400
		return false, ""
	}

	metricSlice := []map[string]interface{}{}
	// transform metrics in map format
	for i := 0; i < len(mrd.Metrics); i++ {
		m := map[string]interface{}{
			"Metricid":            mrd.Metrics[i].MetricID,
			"MetricProperties":    mrd.Metrics[i].MetricProperties,
			"CollectionDuration":  mrd.Metrics[i].CollectionDuration,
			"CollectionFunction":  mrd.Metrics[i].CollectionFunction,
			"CollectionTimeScope": mrd.Metrics[i].CollectionTimeScope,
		}
		metricSlice = append(metricSlice, m)
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
	ts.d.EventBus.PublishEvent(ctx, eh.NewEvent(AddedMRDEvent, &mrd, time.Now()))
	return true, mrduuid
}
