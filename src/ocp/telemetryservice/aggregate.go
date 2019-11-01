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
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Id":   "TelemetryService",
						"Name": "Telemetry Service",
						"Actions": map[string]interface{}{
							"#TelemetryService.SubmitTestMetricReport": map[string]interface{}{
								"target": vw.GetActionURI("submit.test.metric.report"),
							},
						},
						"Oem":                     map[string]interface{}{},
						"MetricReportDefinitions": map[string]interface{}{"@odata.id": vw.GetURI() + "/MetricReportDefinitions"},
						"MetricReports":           map[string]interface{}{"@odata.id": vw.GetURI() + "/MetricReports"},
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
					Type:        "#MetricReportDefinitionCollection.MetricReportDefinitionCollection",
					Context:     "/redfish/v1/$metadata#MetricReportDefinitionCollection.MetricReportDefinitionCollection",
					Plugin:      "TelemetryService",
					Privileges: map[string]interface{}{
						"GET":  []string{"Login"},
						"POST": []string{"ConfigureManager"},
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
					Type:        "#MetricReportCollection.MetricReportCollection",
					Context:     "/redfish/v1/$metadata#MetricReportCollection.MetricReportCollection",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Id":                       "MetricReports",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
					}},
			}, nil
		})

	s.RegisterAggregateFunction("fanspeedmetricdef",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			// TODO:  dynamically manage MetricProperties
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#MetricDefinition.v1_0_0.MetricDefinition",
					Context:     "/redfish/v1/$metadata#MetricDefinition.MetricDefinition",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Id":              "FanSpeed",
						"Name":            "Fan Speed Metric Definition",
						"MetricType":      "Numeric",
						"Implementation":  "PhysicalSensor",
						"PhysicalContext": "Fan",
						"MetricDataType":  "Decimal",
						"MetricProperties": []string{
							"/redfish/v1/Chassis/System.Chassis.1/Sensors/Fans/Fan.Slot.{SlotNumber}#Oem/Reading",
						},
						"Units":             "RPM",
						"Precision":         2,
						"Accuracy":          1.0,
						"Calibration":       2,
						"MinReadingRange":   0.0,
						"MaxReadingRange":   1000.0,
						"SensingInterval":   "PT1S",
						"TimestampAccuracy": "PT1S",
						"Wildcards": []interface{}{
							map[string]interface{}{
								"Name":   "SlotNumber",
								"Values": []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "*"},
							}}}},
			}, nil

		})
}
