package obmc

// this file should define the BMC Manager object golang data structures where
// we put all the data, plus the aggregate that pulls the data.  actual data
// population should happen in an impl class. ie. no dbus calls in this file

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	domain "github.com/superchalupa/go-redfish/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

const (
	BmcPlugin      = domain.PluginType("obmc_bmc")
)

// OCP Profile Redfish BMC object

type bmcService struct {
	// be sure to lock if reading or writing any data in this object
	sync.Mutex

	// Any struct field with tag "property" will automatically be made available in the @meta and will be updated in real time.
	URIName        string `property:"uri_name"`
	Name        string `property:"name"`
	Description string `property:"description"`
	Model       string `property:"model"`
	Timezone    string `property:"timezone"`
	Version     string `property:"version"`
}

func NewBMCService() (*bmcService, error) {
	s := &bmcService{}
	return s, nil
}

func (s *bmcService) GetOdataID() string { return "/redfish/v1/Managers/" + s.URIName }

// satisfy the plugin interface so we can list ourselves as a plugin in our @meta
func (s *bmcService) PluginType() domain.PluginType { return BmcPlugin }

func (s *bmcService) RefreshProperty(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
	body interface{},
) {
	s.Lock()
	defer s.Unlock()

	// Generic ability to use reflection to pull data out of the BMC service
	// object. Anything with a struct tag of "property" is accessible here, in
	// realtime. If you set up a bakcground task to update, it will
	// automatically update on GET
	property, ok := meta["property"].(string)
	if ok {
		v := reflect.ValueOf(*s)
		for i := 0; i < v.NumField(); i++ {
			// Get the field, returns https://golang.org/pkg/reflect/#StructField
			tag := v.Type().Field(i).Tag.Get("property")
			if tag == property {
				rrp.Value = v.Field(i).Interface()
				return
			}
		}
	}
	fmt.Printf("Incorrect metadata in aggregate: neither 'data' nor 'property' set to something handleable")
}


func (s *bmcService) AddOBMCManagerResource(ctx context.Context, ch eh.CommandHandler) {
	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
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
				"Id":                       s.URIName,
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
				"Links": map[string]interface{}{},
				"Actions": map[string]interface{}{
					"#Manager.Reset": map[string]interface{}{
						"target": "/redfish/v1/Managers/bmc/Actions/Manager.Reset",
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
			ResourceURI: "/redfish/v1/Managers/bmc/Actions/Manager.Reset",
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
