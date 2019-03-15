package stdcollections

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

	stdCollectionPrivs := map[string]interface{}{
		"GET":    []string{"Login"},
		"POST":   []string{}, // Read Only
		"PUT":    []string{}, // Read Only
		"PATCH":  []string{}, // Read Only
		"DELETE": []string{}, // can't be deleted
	}

	s.RegisterAggregateFunction("root",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#ServiceRoot.v1_4_0.ServiceRoot",
					Context:     vw.GetURI() + "/$metadata#ServiceRoot.ServiceRoot",

					Privileges: map[string]interface{}{
						"GET": []string{"Unauthenticated"},
					},
					Properties: map[string]interface{}{
						"Id":             "RootService",
						"Name":           "Root Service",
						"Description":    "Root Service",
						"RedfishVersion": "1.1.0",
						"@odata.etag":    `W/"abc123"`,
					}},
			}, nil
		})

	s.RegisterAggregateFunction("chassis",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#ChassisCollection.ChassisCollection",
					Context:     params["rooturi"].(string) + "/$metadata#ChassisCollection.ChassisCollection",
					Privileges:  stdCollectionPrivs,
					Properties: map[string]interface{}{
						"Name":                     "Chassis Collection",
						"Description":              "Collection of Chassis",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
					}},
				&domain.UpdateRedfishResourceProperties{
					ID: params["rootid"].(eh.UUID),
					Properties: map[string]interface{}{
						"Chassis": map[string]interface{}{"@odata.id": vw.GetURI()},
					}},
			}, nil
		})

	s.RegisterAggregateFunction("systems",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#ComputerSystemCollection.ComputerSystemCollection",
					Context:     params["rooturi"].(string) + "/$metadata#ComputerSystemCollection.ComputerSystemCollection",
					Privileges:  stdCollectionPrivs,
					Properties: map[string]interface{}{
						"Name":                     "Computer System Collection",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
					}},
				&domain.UpdateRedfishResourceProperties{
					ID: params["rootid"].(eh.UUID),
					Properties: map[string]interface{}{
						"Systems": map[string]interface{}{"@odata.id": vw.GetURI()},
					}},
			}, nil
		})

	s.RegisterAggregateFunction("managers",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#ManagerCollection.ManagerCollection",
					Context:     params["rooturi"].(string) + "/$metadata#ManagerCollection.ManagerCollection",
					Privileges:  stdCollectionPrivs,
					Properties: map[string]interface{}{
						"Name":                     "ManagerInstancesCollection",
						"Description":              "Collection of BMCs",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
					}},

				&domain.UpdateRedfishResourceProperties{
					ID: params["rootid"].(eh.UUID),
					Properties: map[string]interface{}{
						"Managers": map[string]interface{}{"@odata.id": vw.GetURI()},
					}},
			}, nil
		})

	s.RegisterAggregateFunction("accountservice",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#AccountService.v1_0_2.AccountService",
					Context:     params["rooturi"].(string) + "/$metadata#AccountService.AccountService",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"POST":   []string{"ConfigureManager"}, // cannot create sub objects
						"PUT":    []string{"ConfigureManager"},
						"PATCH":  []string{"ConfigureManager"},
						"DELETE": []string{}, // can't be deleted
					},
					Properties: map[string]interface{}{
						"Id":          "AccountService",
						"Name":        "Account Service",
						"Description": "Account Service",
						"Status": map[string]interface{}{
							"State":  "Enabled",
							"Health": "OK",
						},
						"ServiceEnabled":                  true,
						"AuthFailureLoggingThreshold":     3,
						"MinPasswordLength":               8,
						"AccountLockoutThreshold":         5,
						"AccountLockoutDuration":          30,
						"AccountLockoutCounterResetAfter": 30,
					}},

				&domain.UpdateRedfishResourceProperties{
					ID: params["rootid"].(eh.UUID),
					Properties: map[string]interface{}{
						"AccountService": map[string]interface{}{"@odata.id": vw.GetURI()},
					}},
			}, nil
		})

	s.RegisterAggregateFunction("roles",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#RoleCollection.RoleCollection",
					Context:     params["rooturi"].(string) + "/$metadata#RoleCollection.RoleCollection",
					Privileges:  stdCollectionPrivs,
					Properties: map[string]interface{}{
						"Name":                     "Roles Collection",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
					}},
				&domain.UpdateRedfishResourceProperties{
					ID: params["actsvc_id"].(eh.UUID),
					Properties: map[string]interface{}{
						"Roles": map[string]interface{}{"@odata.id": vw.GetURI()},
					}},
			}, nil
		})

	s.RegisterAggregateFunction("accounts",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#ManagerAccountCollection.ManagerAccountCollection",
					Context:     params["rooturi"].(string) + "/$metadata#ManagerAccountCollection.ManagerAccountCollection",
					Privileges:  stdCollectionPrivs,
					Properties: map[string]interface{}{
						"Name":                     "Accounts Collection",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
					}},
				&domain.UpdateRedfishResourceProperties{
					ID: params["actsvc_id"].(eh.UUID),
					Properties: map[string]interface{}{
						"Accounts": map[string]interface{}{"@odata.id": vw.GetURI()},
					}},
			}, nil
		})

}

func AddStandardRoles(ctx context.Context, rootID eh.UUID, rootURI string, ch eh.CommandHandler) {
	// Create Computer System Collection

	// add standard DMTF roles: Admin
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID: eh.NewUUID(),

			ResourceURI: rootURI + "/AccountService/Roles/Admin",
			Type:        "#Role.v1_0_2.Role",
			Context:     rootURI + "/$metadata#Role.Role",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // Read Only
				"PUT":    []string{}, // Read Only
				"PATCH":  []string{}, // Read Only
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name":         "User Role",
				"Id":           "Admin",
				"Description":  "Admin User Role",
				"IsPredefined": true,
				"AssignedPrivileges": []string{
					"Login",
					"ConfigureManager",
					"ConfigureUsers",
					"ConfigureSelf",
					"ConfigureComponents",
				},
			}})

	// add standard DMTF roles: Operator
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			ResourceURI: rootURI + "/AccountService/Roles/Operator",
			Type:        "#Role.v1_0_2.Role",
			Context:     rootURI + "/$metadata#Role.Role",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // Read Only
				"PUT":    []string{}, // Read Only
				"PATCH":  []string{}, // Read Only
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name":         "User Role",
				"Id":           "Operator",
				"Description":  "Operator User Role",
				"IsPredefined": true,
				"AssignedPrivileges": []string{
					"Login",
					"ConfigureSelf",
					"ConfigureComponents",
				},
			}})

	// add standard DMTF roles: Read-only
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			ResourceURI: rootURI + "/AccountService/Roles/ReadOnlyUser",
			Type:        "#Role.v1_0_2.Role",
			Context:     rootURI + "/$metadata#Role.Role",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // Read Only
				"PUT":    []string{}, // Read Only
				"PATCH":  []string{}, // Read Only
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name":         "User Role",
				"Id":           "ReadOnlyUser",
				"Description":  "ReadOnlyUser User Role",
				"IsPredefined": true,
				"AssignedPrivileges": []string{
					"Login",
				},
			}})

}
