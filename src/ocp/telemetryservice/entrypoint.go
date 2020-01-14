package telemetryservice

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	"github.com/superchalupa/sailfish/src/ocp/eventservice"
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
	sync.RWMutex
	d             *domain.DomainObjects
	ew            waiter
	ch            eh.CommandHandler
	mrdConfigL    []mrdConfig
	metric2Report map[string][]*mrdConfig
	logger        log.Logger
}

type mrdConfig struct {
	name    string
	mrURI   string
	mrUUID  eh.UUID
	mrdURI  string
	mrdUUID eh.UUID
	config  mrdPatch
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
	d.EventBus.AddHandler(eh.MatchAnyEventOf(MetricValueEvent, domain.RedfishResourceRemoved, domain.RedfishResourcePropertiesUpdated2, domain.RedfishResourceCreated), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Telemetry Service"), eventwaiter.NoAutoRun)
	EventPublisher.AddObserver(EventWaiter)
	go EventWaiter.Run()

	ret := &TelemetryService{
		d:             d,
		ew:            EventWaiter,
		ch:            chdler,
		mrdConfigL:    []mrdConfig{},
		metric2Report: map[string][]*mrdConfig{}, // MetricProperties : []&mrdConfig, ex: System.Chassis.1/Thermal/Fan.Slot.1#Reading: []&mrdConfig
		logger:        logger,
	}

	ret.StartTelemetryService(ctx)
	return ret
}

func (ts *TelemetryService) setMRDConfig(Id string, mrUUID eh.UUID, mrdUUID eh.UUID, mrURI string, mrdURI string, mrdEnabled bool, mrdType string, PropL []string) {
	var mrdP *mrdConfig = nil
	for i := 0; i < len(ts.mrdConfigL); i++ {
		if ts.mrdConfigL[i].name == Id {
			mrdP = &ts.mrdConfigL[i]
			break
		}
	}

	ts.Lock()
	defer ts.Unlock()
	if mrdP == nil {
		ts.mrdConfigL = append(ts.mrdConfigL, mrdConfig{
			name:    Id,
			mrUUID:  mrUUID,
			mrdUUID: mrdUUID,
			mrURI:   mrURI,
			mrdURI:  mrdURI,
			config: mrdPatch{
				mrdEnabled: mrdEnabled,
				mrdType:    mrdType,
			}})
		mrdP = &ts.mrdConfigL[len(ts.mrdConfigL)-1]

	} else {
		ts.deleteMRDConfig(mrdP)

		mrdP.name = Id
		mrdP.mrdUUID = mrdUUID
		mrdP.mrdURI = mrdURI
		mrdP.config.mrdEnabled = mrdEnabled
		mrdP.config.mrdType = mrdType
		mrdP.mrUUID = mrUUID
		mrdP.mrURI = mrURI
	}

	for i := 0; i < len(PropL); i++ {
		pS := PropL[i]
		_, ok := ts.metric2Report[pS]
		if ok {
			ts.metric2Report[pS] = append(ts.metric2Report[pS], mrdP)
		} else {
			ts.metric2Report[pS] = []*mrdConfig{mrdP}
		}
	}

}

func (ts *TelemetryService) deleteMRDConfig(mrdP *mrdConfig) {
	for key, val := range ts.metric2Report {
		for i := 0; i < len(val); i++ {
			if val[i] == mrdP {
				if len(val) == 1 {
					val[i] = nil
					val = nil
				} else {
					tmp := val[i]
					val[i] = val[0]
					val[0] = tmp
					val[0] = nil

					val = val[1:]
				}

			}

			if len(val) == 0 {
				delete(ts.metric2Report, key)
			}
		}

	}
}

func (ts *TelemetryService) sendMetricEvent(ctx context.Context, mrdUUID eh.UUID, metricID string, metricValue interface{}, metricProp string, metricType string) {
	valS := fmt.Sprintf("%+v", metricValue)

	eventData := &MetricValueEventData{
		UUID:     mrdUUID,
		MetricId: metricID,
		//HSM TODO add other type conversion to string
		MetricValue: valS,
		Timestamp:   time.Now().UTC().Format("2006-01-02T15:04:05-07:00"),

		MetricProperty:   metricProp,
		reportUpdateType: metricType,
	}
	ts.d.EventBus.PublishEvent(ctx, eh.NewEvent(MetricValueEvent, eventData, time.Now()))
}

// will create a model, view, and controller for the subscription
//      If you want to save settings, hook up a mapper to the "default" view returned
func (ts *TelemetryService) StartTelemetryService(ctx context.Context) error {
	//TODO HSM optimize
	eh.RegisterCommand(func() eh.Command {
		return &POST{ts: ts, d: ts.d}
	})
	listener, err := ts.ew.Listen(ctx,
		func(event eh.Event) bool {
			switch typ := event.EventType(); typ {
			case domain.RedfishResourcePropertiesUpdated2:
				// first match url.  Then match property.  then send metric event for each metricreportdefinition.
				if data, ok := event.Data().(*domain.RedfishResourcePropertiesUpdatedData2); ok {
					// update MR/MRD config here

					ts.RLock()
					for mProp, MRDConfigL := range ts.metric2Report {
						mPropSplit := strings.Split(mProp, "#")
						if len(mPropSplit) != 2 {
							ts.logger.Error("Telemetry: Bad MRD Property", mPropSplit)
							continue
						}

						mURL := mPropSplit[0]
						mPropPath := mPropSplit[1]
						if mURL != data.ResourceURI {
							continue
						}
						for aPath, val := range data.PropertyNames {

							if aPath == mPropPath {
								for i := 0; i < len(MRDConfigL); i++ {
									// send every metric change
									if MRDConfigL[i].config.mrdType != "OnChange" {
										continue
									}

									if MRDConfigL[i].config.mrdEnabled {
										ts.sendMetricEvent(ctx, MRDConfigL[i].mrUUID, MRDConfigL[i].name, val, mProp, MRDConfigL[i].config.mrdType)
									}
								}
								break
							}
						}
					}
					ts.RUnlock()
				}
			case MetricValueEvent:
				return true
			case domain.RedfishResourceRemoved:
				if data, ok := event.Data().(*domain.RedfishResourceRemovedData); ok {
					if strings.Contains(data.ResourceURI, "/redfish/v1/TelemetryService/MetricReportDefinitions/") {
						var duri string
						var duuid eh.UUID
						var mrdP *mrdConfig
						for i := 0; i < len(ts.mrdConfigL); i++ {
							if data.ResourceURI == ts.mrdConfigL[i].mrURI {

								duri = ts.mrdConfigL[i].mrURI
								duuid = ts.mrdConfigL[i].mrUUID
								mrdP = &ts.mrdConfigL[i]
								break
							}
						}
						if duri != "" {
							ts.deleteMRDConfig(mrdP)
							ts.d.CommandHandler.HandleCommand(ctx, &domain.RemoveRedfishResource{ID: duuid, ResourceURI: duri})
						}
					}
				}

			}

			return false
		},
	)

	if err != nil {
		return nil
	}

	// current design train of thought.  Having the aggregate updated here and metric event sent here allows more freedom to
	// handle scheduling, grouping changes then updating/sending
	go func() {
		APPENDLIMIT := 150
		// delete the aggregate
		defer listener.Close()

		for {
			select {
			case event := <-listener.Inbox():
				if e, ok := event.(syncEvent); ok {
					e.Done()
				}
				switch typ := event.EventType(); typ {
				case MetricValueEvent:
					var data *MetricValueEventData
					var ok bool
					if data, ok = event.Data().(*MetricValueEventData); !ok {
						ts.logger.Warn(fmt.Sprintf("Type is %T  not *MetricValueEventData\n", event.Data()))
						continue
					}
					valItem := map[string]interface{}{
						"MetricId":       "",
						"MetricValue":    data.MetricValue,
						"Timestamp":      data.Timestamp,
						"MetricProperty": data.MetricProperty,
					}
					valL := []interface{}{valItem}
					for i := 0; i < len(ts.metric2Report[data.MetricProperty]); i++ {

						// can batch MetricValueEvent saves..
						ts.d.CommandHandler.HandleCommand(ctx,
							&domain.UpdateMetricRedfishResource{
								ID:               data.UUID,
								AppendLimit:      APPENDLIMIT,
								ReportUpdateType: data.reportUpdateType,
								Properties: map[string]interface{}{
									"MetricValues": valL,
								},
							})
					}
					agg, _ := ts.d.AggregateStore.Load(ctx, domain.AggregateType, data.UUID)
					redfishResource, ok := agg.(*domain.RedfishResourceAggregate)
					results := domain.Flatten(&redfishResource.Properties, false)
					resultD := results.(map[string]interface{})
					eventData := eventservice.MetricReportData{Data: resultD}
					ts.d.EventBus.PublishEvent(ctx, eh.NewEvent(eventservice.ExternalMetricEvent, eventData, time.Now()))

				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (ts *TelemetryService) CreateMetricReportDefinition(ctx context.Context, mrd MetricReportDefinition, data *domain.HTTPCmdProcessedData) (bool, eh.UUID, eh.UUID) {
	log.ContextLogger(ctx, "submit_mrd").Debug("got test metric report event", "event_data", mrd)
	errmmsg := ""
	errmFmt := "%s, "

	if mrd.Id == "" {
		errmmsg += fmt.Sprintf(errmFmt, "Id")
	}

	if len(mrd.MetricProperties) == 0 {
		errmmsg += fmt.Sprintf(errmFmt, "MetricProperties")
	}

	if mrd.MetricReportDefinitionType == "" {
		errmmsg += fmt.Sprintf(errmFmt, "MetricReportDefinitionType")
	}
	if errmmsg != "" {
		data.Results = map[string]interface{}{"msg": "Metric Report Definition Properties are not present: " + errmmsg}
		data.StatusCode = 400
		return false, "", ""
	}

	mruuid := eh.NewUUID()
	mrdURL := "/redfish/v1/TelemetryService/MetricReportDefinitions/" + mrd.Id
	mrURL := "/redfish/v1/TelemetryService/MetricReports/" + mrd.Id

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
				"SuppressRepeatedMetricValue@meta": map[string]interface{}{
					"DEFAULT": mrd.SuppressRepeatedMetricValue,
					"PATCH": map[string]interface{}{
						"plugin": "GenericBool"}},
				"MetricReportHeartbeatInterval": mrd.MetricReportHeartbeatInterval,
				"Wildcards":                     mrd.Wildcards,
				"MetricProperties":              mrd.MetricProperties,
				"ReportUpdates":                 "AppendWrapsWhenFull",
				"ReportActions": []string{
					"RedfishEvent", "LogToMetricReportsCollection"},
				"MetricReport": map[string]interface{}{
					"@odata.id": mrURL,
				},
			},
		})

	ts.setMRDConfig(mrd.Id, mruuid, mrduuid, mrURL, mrdURL, mrd.MetricReportDefinitionEnabled, mrd.MetricReportDefinitionType, mrd.MetricProperties)

	// Metric Report URL is provided with MRD, therefore creating Metric Report URL first
	ts.ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          mruuid,
			ResourceURI: mrURL,
			Type:        "#MetricReport.v1_0_1.MetricReport",
			Context:     "/redfish/v1/$metadata#MetricReport.MetricReport",
			Privileges: map[string]interface{}{
				"GET": []string{"Login"},
			},
			Properties: map[string]interface{}{
				"Id":             mrd.Id,
				"Description":    mrd.Description,
				"Name":           "",
				"ReportSequence": "",
				"MetricReportDefinition": map[string]interface{}{
					"@odata.id": mrdURL,
				},
				"MetricValues": []map[string]interface{}{},
			},
		})

	default_msg := domain.ExtendedInfo{}

	data.StatusCode = 201
	data.Results = default_msg.GetDefaultExtendedInfo()
	return true, mruuid, mrduuid
}
