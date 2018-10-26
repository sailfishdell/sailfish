package testaggregate

// this file should define the BMC Manager object golang data structures where
// we put all the data, plus the aggregate that pulls the data.  actual data
// population should happen in an impl class. ie. no dbus calls in this file

import (
	"context"
	"sync"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func RegisterAggregate(s *Service) {
	s.RegisterAggregateFunction("testview",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{

					ResourceURI: vw.GetURI(),
					Type:        "#Manager.v1_0_2.Manager",
					Context:     "/redfish/v1/$metadata#Manager.Manager",
					Privileges: map[string]interface{}{
						"GET":   []string{"Login"},
						"PATCH": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Id@meta":          vw.Meta(view.PropGET("unique_name")),
						"Name@meta":        vw.Meta(view.PropGET("name"), view.PropPATCH("name", "ar_mapper")),
						"Description@meta": vw.Meta(view.PropGET("description")),
						"Model@meta":       vw.Meta(view.PropGET("model"), view.PropPATCH("model", "ar_mapper")),
					}}}, nil
		})
}
