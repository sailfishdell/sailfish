package registries

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
	s.RegisterAggregateFunction("registry_collection",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#MessageRegistryFileCollection.MessageRegistryFileCollection",
					Context:     params["rooturi"].(string) + "/$metadata#MessageRegistryFileCollection.MessageRegistryFileCollection",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Description":              "Registry Repository",
						"Name":                     "Registry File Collection",
						"Members":                  []interface{}{},
						"Members@odata.count":      0,
					}},
				&domain.UpdateRedfishResourceProperties{
					ID: params["rootid"].(eh.UUID),
					Properties: map[string]interface{}{
						"Registries": map[string]interface{}{"@odata.id": vw.GetURI()},
					}},
			}, nil
		})

	s.RegisterAggregateFunction("registry",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ID:          vw.GetUUID(),
					ResourceURI: vw.GetURI(),
					Type:        "#MessageRegistryFile.v1_0_2.MessageRegistryFile",
					Context:     "/redfish/v1/$metadata#MessageRegistryFile.MessageRegistryFile",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Id@meta":                    vw.Meta(view.GETProperty("id"), view.GETModel("default")),
						"Name@meta":                  vw.Meta(view.GETProperty("name"), view.GETModel("default")),
						"Description@meta":           vw.Meta(view.GETProperty("description"), view.GETModel("default")),
						"Registry@meta":              vw.Meta(view.GETProperty("type"), view.GETModel("default")),
						"Languages@meta":             vw.Meta(view.GETProperty("languages"), view.GETModel("default")),
						"Languages@odata.count@meta": vw.Meta(view.GETProperty("languages"), view.GETFormatter("count"), view.GETModel("default")),
						"Location@meta":              vw.Meta(view.GETProperty("location"), view.GETModel("default")),
						"Location@odata.count@meta":  vw.Meta(view.GETProperty("location"), view.GETFormatter("count"), view.GETModel("default")),
					}},
			}, nil
		})
}
