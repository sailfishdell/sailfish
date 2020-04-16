package fans

import (
	"context"
	"sync"

	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func RegisterAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("fan",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Thermal.v1_0_2.Fan",
					Context:     "/redfish/v1/$metadata#Thermal.Thermal",
					Privileges: map[string]interface{}{
						"GET":   []string{"Login"},
						"PATCH": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Description":   "Represents the properties for Fan and Cooling",
						"FanName@meta":  vw.Meta(view.PropGET("fanname")),
						"MemberId@meta": vw.Meta(view.PropGET("unique_name")),
						"ReadingUnits":  "RPM",
						"Reading":       0,
						"Status": map[string]interface{}{
							"HealthRollup": "OK",
							"Health":       "OK",
						},
						"Oem": map[string]interface{}{
							"Dell": map[string]interface{}{
								"Attributes@meta": vw.Meta(view.GETProperty("attributes"), view.GETFormatter("attributeFormatter"), view.GETModel("default"), view.PropPATCH("attributes", "ar_dump")),
							},
							"ReadingUnits":         "Percent",
							"Reading":              0,
							"FirmwareVersion@meta": vw.Meta(view.PropGET("firmware_version")),
							"HardwareVersion@meta": vw.Meta(view.PropGET("hardware_version")),
							"GraphicsURI@meta":     vw.Meta(view.PropGET("graphics_uri")),
						},
					},
				}}, nil
		})
}
