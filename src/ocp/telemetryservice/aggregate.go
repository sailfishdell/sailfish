package telemetryservice

import (
	"context"

	"github.com/superchalupa/go-redfish/src/ocp/view"

	"github.com/superchalupa/go-redfish/src/log"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, rootID eh.UUID, ch eh.CommandHandler, eb eh.EventBus) {
	// Create SessionService aggregate
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			ResourceURI: v.GetURI(),
			Type:        "#TelemetryService.v1_0_0.TelemetryService",
			Context:     "/redfish/v1/$metadata#TelemetryService.TelemetryService",
			Privileges: map[string]interface{}{
				"GET":    []string{"ConfigureManager"},
				"POST":   []string{},
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{},
			},
			Properties: map[string]interface{}{
				"Id":             "TelemetryService",
				"Name":           "Telemetry Service",
				"ServiceEnabled": true,
				"Actions": map[string]interface{}{
					"#TelemetryService.SubmitTestMetricReport": map[string]interface{}{
						"target": v.GetActionURI("submit.test.metric.report"),
					},
					"Oem": map[string]interface{}{},
				},
			}})

	ch.HandleCommand(ctx,
		&domain.UpdateRedfishResourceProperties{
			ID: rootID,
			Properties: map[string]interface{}{
				"TelemetryService": map[string]interface{}{"@odata.id": "/redfish/v1/TelemetryService"},
			},
		})

	return
}
