package slots

import (
	"context"
	"strings"

	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"

	"github.com/superchalupa/sailfish/src/log"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func RegisterAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("slotcollection",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#DellSlotsCollection.DellSlotsCollection",
					Context:     params["rooturi"].(string) + "/$metadata#DellSlotsCollection.DellSlotsCollection",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Name":                     "DellSlotsCollection",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
					}},
			}, nil
		})

	s.RegisterAggregateFunction("slot",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {

			properties := map[string]interface{}{
				"Id":            params["FQDD"],
				"Name@meta":     vw.Meta(view.PropGET("slot_name")),
				"SlotName@meta": vw.Meta(view.PropGET("slot_slotname")),
				"Occupied@meta": vw.Meta(view.PropGET("slot_occupied")),
				"Config@meta":   vw.Meta(view.PropGET("slot_config")),
				"Contains@meta": vw.Meta(view.PropGET("slot_contains")),
			}

			if strings.Contains(params["FQDD"].(string), "SledSlot") {
				properties["SledProfile@meta"] = vw.Meta(view.PropGET("sled_profile"))
			}

			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#DellSlot.v1_0_0.DellSlot",
					Context:     params["rooturi"].(string) + "/$metadata#DellSlot.DellSlot",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: properties,
				},
			}, nil
		})
}
