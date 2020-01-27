package telemetryservice

import (
	"context"
	"errors"
	"fmt"
	"time"
	//"strings"

	eh "github.com/looplab/eventhorizon"

	ah "github.com/superchalupa/sailfish/src/actionhandler"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/eventservice"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func MakeSubmitTestMetricReport(eb eh.EventBus, d *domain.DomainObjects, ch eh.CommandHandler) func(context.Context, eh.Event, *domain.HTTPCmdProcessedData) error {

	return func(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
		log.ContextLogger(ctx, "submit_event").Debug("got test metric report event", "event_data", event.Data())

		d, ok := event.Data().(*ah.GenericActionEventData)
		if !ok {
			log.ContextLogger(ctx, "submit_event").Crit("type assert failed", "event_data", event.Data(), "Type", fmt.Sprintf("%T", event.Data()))
			return errors.New("didnt get the right kind of event")
		}

		m, ok := d.ActionData.(map[string]interface{})
		if !ok {
			log.ContextLogger(ctx, "submit_event").Crit("type assert failed", "event data is not a map[string] interface", d.ActionData, "Type", fmt.Sprintf("%T", d.ActionData))
			return errors.New("didnt get the right kind of event")
		}

		name, ok := m["MetricName"]
		if !ok {
			log.ContextLogger(ctx, "submit_event").Crit("metric report name is not provided", "event_data", event.Data())
			retData.StatusCode = 400
			retData.Results = map[string]interface{}{"msg": "MetricName is not present"}
			return errors.New("metric report name is not provided")
		}
		n, ok := name.(string)
		if !ok {
			log.ContextLogger(ctx, "submit_event").Crit("metric report name is not a string", "event_data", event.Data())
			retData.StatusCode = 400
			retData.Results = map[string]interface{}{"msg": "MetricName is not a string"}
			return errors.New("metric report name is not a string")
		}

		mVal, ok := m["MetricValues"]
		if !ok {
			log.ContextLogger(ctx, "submit_event").Crit("metric report values is not provided", "event_data", event.Data())
			retData.StatusCode = 400
			retData.Results = map[string]interface{}{"msg": "MetricValues is not present"}
			// I wonder what the default is if an error returns
			return errors.New("metric report value is not provided")
		}

		uuid := eh.NewUUID()

		ch.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          uuid,
				ResourceURI: "/redfish/v1/TelemetryService/MetricReports/" + n,
				Type:        "#MetricReport.v1_0_0.MetricReport",
				Context:     "/redfish/v1/$metadata#MetricReport.MetricReport",
				Privileges: map[string]interface{}{
					"GET":    []string{"Login"},
					"DELETE": []string{"ConfigureManager"},
				},
				Properties: map[string]interface{}{
					"Id":           n,
					"Name":         n,
					"Timestamp":    time.Now().UTC().Format(time.RFC3339),
					"MetricValues": mVal,
					"MetricReportDefinition": map[string]interface{}{
						"@odata.id": "/redfish/v1/TelemetryService/MetricReportDefinitions"},
				}})

		eventData := eventservice.MetricReportData{Data: m}
		responseEvent := eh.NewEvent(eventservice.ExternalMetricEvent, eventData, time.Now())
		eb.PublishEvent(ctx, responseEvent)

		retData.Headers = map[string]string{
			"Location": "/redfish/v1/TelemetryService/MetricReports/" + n,
		}
		default_msg := domain.ExtendedInfo{}
		retData.Results = default_msg.GetDefaultExtendedInfo()
		retData.StatusCode = 201
		return nil
	}
}
