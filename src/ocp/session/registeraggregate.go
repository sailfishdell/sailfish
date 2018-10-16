package session

import (
	"context"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func RegisterAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("sessionservice",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#SessionService.v1_0_2.SessionService",
					Context:     "/redfish/v1/$metadata#SessionService.SessionService",
					Privileges: map[string]interface{}{
						"GET":   []string{"ConfigureManager"},
						"PATCH": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Id":          "SessionService",
						"Name":        "Session Service",
						"Description": "Session Service",
						"Status": map[string]interface{}{
							"State":  "Enabled",
							"Health": "OK",
						},
						"ServiceEnabled":      true,
						"SessionTimeout@meta": vw.Meta(view.GETProperty("session_timeout"), view.GETModel("default"), view.PropPATCH("session_timeout", "default")),
						"Sessions": map[string]interface{}{
							"@odata.id": "/redfish/v1/SessionService/Sessions",
						},
					},
				},
				&domain.UpdateRedfishResourceProperties{
					ID: params["rootid"].(eh.UUID),
					Properties: map[string]interface{}{
						"SessionService": map[string]interface{}{"@odata.id": vw.GetURI()},
					}},
			}, nil
		})

	s.RegisterAggregateFunction("sessioncollection",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#SessionCollection.SessionCollection",
					Context:     "/redfish/v1/$metadata#SessionCollection.SessionCollection",
					Plugin:      "SessionService",
					Privileges: map[string]interface{}{
						"GET":    []string{"ConfigureManager"},
						"POST":   []string{"Unauthenticated"},
						"PUT":    []string{"ConfigureManager"},
						"PATCH":  []string{"ConfigureManager"},
						"DELETE": []string{"ConfigureSelf"},
					},
					Properties: map[string]interface{}{
						"Name":                     "Session Collection",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
					},
				},
				&domain.UpdateRedfishResourceProperties{
					ID: params["rootid"].(eh.UUID),
					Properties: map[string]interface{}{
						"Links": map[string]interface{}{"Sessions": map[string]interface{}{"@odata.id": "/redfish/v1/SessionService/Sessions"}},
					}},
			}, nil
		})

	s.RegisterAggregateFunction("session",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Session.v1_0_0.Session",
					Context:     "/redfish/v1/$metadata#Session.Session",
					Privileges: map[string]interface{}{
						"GET":    []string{"ConfigureManager"},
						"POST":   []string{"ConfigureManager"},
						"PUT":    []string{"ConfigureManager"},
						"PATCH":  []string{"ConfigureManager"},
						"DELETE": []string{"ConfigureSelf_" + params["username"].(string), "ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Id":          vw.GetUUID(),
						"Name":        "User Session",
						"Description": "User Session",
						"UserName":    params["username"],
					}},
			}, nil
		})

}
