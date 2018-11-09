package idrac_embedded

import (
	"context"
	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"sync"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func RegisterAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("idrac_embedded",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:         "#Manager.v1_0_2.Manager",
					Context:     "/redfish/v1/$metadata#Manager.Manager",

					Privileges: map[string]interface{}{
						"GET": []string{"Unauthenticated"},
					},
					Properties: map[string]interface{}{
						"@odata.etag": `W/"abc123"`,

						// replace with model calls...
						"Model":               "14G Monolithic",
						"DateTimeLocalOffset": "-05:00",
						"UUID":                "3132334f-c0b7-3480-3510-00364c4c4544",
						"Name":                "Manager",
						"@odata.type":         "#Manager.v1_0_2.Manager",
						"FirmwareVersion":     "3.15.15.15",
						"ManagerType":         "BMC",
					}},
			}, nil
		})
}
