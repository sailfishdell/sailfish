package bmc

// this file should define the BMC Manager object golang data structures where
// we put all the data, plus the aggregate that pulls the data.  actual data
// population should happen in an impl class. ie. no dbus calls in this file

import (
	"context"

	"github.com/superchalupa/go-redfish/plugins"
	domain "github.com/superchalupa/go-redfish/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

const (
	BmcPlugin = domain.PluginType("obmc_bmc")
)

// OCP Profile Redfish BMC object

type service struct {
	*plugins.Service
}


func NewBMCService(options ...interface{}) (*service, error) {
	s := &service{
		Service: plugins.NewService(plugins.PluginType(BmcPlugin)),
	}
    s.ApplyOption(plugins.UUID())
	s.ApplyOption(options...)
    s.ApplyOption(plugins.PropertyOnce("uri", "/redfish/v1/Managers/" + s.GetProperty("unique_name").(string)))
	return s, nil
}

func WithUniqueName(uri string) plugins.Option {
    return plugins.PropertyOnce("unique_name", uri)
}

func (s *service) RefreshProperty(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
	body interface{},
) {
	s.Lock()
	defer s.Unlock()
	err := plugins.RefreshProperty(ctx, *s, rrp, meta)
	if err != nil {
		s.Service.RefreshProperty_unlocked(ctx, agg, rrp, method, meta, body)
	}
}

func (s *service) AddResource(ctx context.Context, ch eh.CommandHandler) {
	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          s.GetUUID(),
			Collection:  false,
			ResourceURI: s.GetOdataID(),
			Type:        "#Manager.v1_1_0.Manager",
			Context:     "/redfish/v1/$metadata#Manager.Manager",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{"ConfigureManager"},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id":                       s.GetProperty("unique_name"),
				"Name@meta":                map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "name"}},
				"ManagerType":              "BMC",
				"Description@meta":         map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "description"}},
				"Model@meta":               map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "model"}},
				"DateTime@meta":            map[string]interface{}{"GET": map[string]interface{}{"plugin": "datetime"}},
				"DateTimeLocalOffset@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "timezone"}},
				"FirmwareVersion@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "version"}},
				"Links":                map[string]interface{}{},

                // Commented out until we figure out what these are supposed to be
				//"ServiceEntryPointUUID":    eh.NewUUID(),
				//"UUID":                     eh.NewUUID(),

				"Status": map[string]interface{}{
					"State":  "Enabled",
					"Health": "OK",
				},
				"Actions": map[string]interface{}{
					"#Manager.Reset": map[string]interface{}{
						"target": s.GetOdataID() + "/Actions/Manager.Reset",
						"ResetType@Redfish.AllowableValues": []string{
							"ForceRestart",
							"GracefulRestart",
						},
					},
				},
			}})

	// handle action for restart
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			ResourceURI: s.GetOdataID() + "/Actions/Manager.Reset",
			Type:        "Action",
			Context:     "Action",
			Plugin:      "GenericActionHandler",
			Privileges: map[string]interface{}{
				"POST": []string{"ConfigureManager"},
			},
			Properties: map[string]interface{}{},
		},
	)
}
