package storage_volume_collection

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

//func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler) {
//	ch.HandleCommand(
//		ctx,
//		&domain.CreateRedfishResource{
//			ID: v.GetUUID(),
//			// Collection:  true, // FIXME: collection code removed, move this to awesome mapper collection instead
//			ResourceURI: v.GetURI(),
//			Type:        "#VolumeCollection.VolumeCollection",
//			Context:     "/redfish/v1/$metadata#VolumeCollection.VolumeCollection",
//			Privileges: map[string]interface{}{
//				"GET":  []string{"Login"},
//				"POST": []string{"ConfigureManager"},
//			},
//			Properties: map[string]interface{}{
//				"Description@meta": v.Meta(view.PropGET("description")), //Done
//				"Name@meta":        v.Meta(view.PropGET("name")),        //Done
//			}})
//}

func RegisterAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("storage_volume_collection",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#VolumeCollection.VolumeCollection",
					Context:     params["rooturi"].(string) + "/$metadata#VolumeCollection.VolumeCollection",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Name":                     "Storage Volume Collection",
						"Description":              "Collection of Storage Volumes",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
					}},
			}, nil
		})

	return
}
