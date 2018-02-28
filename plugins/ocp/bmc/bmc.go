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
	id eh.UUID

	// Any struct field with tag "property" will automatically be made available in the @meta and will be updated in real time.
	uriName string
}

type Option func(*service) error

func NewBMCService(options ...Option) (*service, error) {
	s := &service{
		Service: plugins.NewService(plugins.PluginType(BmcPlugin)),
		id:      eh.NewUUID(),
	}
	s.ApplyOption(options...)
	return s, nil
}

func (c *service) ApplyOption(options ...Option) error {
	for _, o := range options {
		err := o(c)
		if err != nil {
			return err
		}
	}
	return nil
}

func WithURIName(uri string) Option {
	return func(b *service) error {
		if b.uriName != "" {
			panic("Cannot reset URI Name once set")
		}
		b.uriName = uri
		return nil
	}
}

func (s *service) GetUUID() eh.UUID   { return s.id }
func (s *service) GetOdataID() string { return "/redfish/v1/Managers/" + s.uriName }

func (s *service) RefreshProperty(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
	body interface{},
) {
	s.Lock()
	err := plugins.RefreshProperty(ctx, *s, rrp, meta)
	s.Unlock()
	if err != nil {
		s.Service.RefreshProperty(ctx, agg, rrp, method, meta, body)
	}
}

func (s *service) AddResource(ctx context.Context, ch eh.CommandHandler) {
	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          s.id,
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
				"Id":                       s.uriName,
				"Name@meta":                map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "name"}},
				"ManagerType":              "BMC",
				"Description@meta":         map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "description"}},
				"ServiceEntryPointUUID":    eh.NewUUID(),
				"UUID":                     eh.NewUUID(),
				"Model@meta":               map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "model"}},
				"DateTime@meta":            map[string]interface{}{"GET": map[string]interface{}{"plugin": "datetime"}},
				"DateTimeLocalOffset@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "timezone"}},
				"Status": map[string]interface{}{
					"State":  "Enabled",
					"Health": "OK",
				},
				"FirmwareVersion@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "version"}},
				"Links":                map[string]interface{}{},
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
