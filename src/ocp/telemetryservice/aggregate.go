package telemetryservice

import (
	"context"
	"sync"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func RegisterAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("telemetry_service",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#TelemetryService.v1_0_0.TelemetryService",
					Context:     "/redfish/v1/$metadata#TelemetryService.TelemetryService",
					Privileges: map[string]interface{}{
						"GET": []string{"Unauthenticated"},
					},
					Properties: map[string]interface{}{
						"Id":             "TelemetryService",
						"Name":           "Telemetry Service",
						"ServiceEnabled": true,
						"Actions": map[string]interface{}{
							"#TelemetryService.SubmitTestMetricReport": map[string]interface{}{
								"target": vw.GetActionURI("submit.test.metric.report"),
							},
							"Oem": map[string]interface{}{},
							"MetricReportDefinitions": map[string]interface{}{"@odata.id": vw.GetURI() + "/MetricReportDefinitions"},
							"MetricReports":           map[string]interface{}{"@odata.id": vw.GetURI() + "/MetricReports"},
						},
					}},

				&domain.UpdateRedfishResourceProperties{
					ID: params["rootid"].(eh.UUID),
					Properties: map[string]interface{}{
						"TelemetryService": map[string]interface{}{"@odata.id": "/redfish/v1/TelemetryService"},
					},
				},
			}, nil
		})

	s.RegisterAggregateFunction("metric_report_definitions",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#TelemetryService.v1_0_0.___TODO___FIXME___",
					Context:     "/redfish/v1/$metadata#TelemetryService.___TODO___FIXME___",
					Privileges: map[string]interface{}{
						"GET": []string{"Unauthenticated"},
					},
					Properties: map[string]interface{}{
						"Id":                       "MetricReportDefinitions",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
					}},
			}, nil
		})

	s.RegisterAggregateFunction("metric_reports",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#TelemetryService.v1_0_0.___TODO___FIXME___",
					Context:     "/redfish/v1/$metadata#TelemetryService.___TODO___FIXME___",
					Privileges: map[string]interface{}{
						"GET": []string{"Unauthenticated"},
					},
					Properties: map[string]interface{}{
						"Id":                       "MetricReports",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
					}},
			}, nil
		})

	return
}
