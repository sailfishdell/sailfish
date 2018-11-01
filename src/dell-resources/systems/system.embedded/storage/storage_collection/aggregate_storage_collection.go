package storage_collection

import (
	"context"
	"sync"

	"github.com/spf13/viper"
	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
)


//func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler) {
//	ch.HandleCommand(
//		ctx,
//		&domain.CreateRedfishResource{
//			ID: v.GetUUID(),
//			// Collection:  true, // FIXME: collection code removed, move this to awesome mapper collection instead
//			ResourceURI: v.GetURI(),
//			Type:        "#StorageCollection.StorageCollection",
//			Context:     "/redfish/v1/$metadata#StorageCollection.StorageCollection",
//			Privileges: map[string]interface{}{
//				"GET": []string{"Login"},
//			},
//			Properties: map[string]interface{}{
//				"Description@meta": v.Meta(view.PropGET("description")), //Done
//				"Name@meta":        v.Meta(view.PropGET("name")),        //Done
//			}})
//}


func RegisterAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("storage_collection",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#StorageCollection.StorageCollection",
					Context:     params["rooturi"].(string) + "/$metadata#StorageCollection.StorageCollection",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Name":                     "Storage  Collection",
						"Description":              "Collection of Storage Devices",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
					}},
			}, nil
		})


	return
}
