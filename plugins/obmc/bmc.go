package obmc

// this file should define the BMC Manager object golang data structures where
// we put all the data, plus the aggregate that pulls the data.  actual data
// population should happen in an impl class. ie. no dbus calls in this file

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/superchalupa/go-redfish/plugins"
	domain "github.com/superchalupa/go-redfish/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

var (
	BmcPlugin      = domain.PluginType("obmc_bmc")
	ProtocolPlugin = domain.PluginType("protocol")

	DbusTimeout time.Duration = 1
)

// OCP Profile Redfish BMC object

type protocolList map[string]protocol
type protocol struct {
	enabled bool
	port    int
	config  map[string]interface{}
}

type bmcService struct {
	// be sure to lock if reading or writing any data in this object
	sync.Mutex

	// Any struct field with tag "property" will automatically be made available in the @meta and will be updated in real time.
	Name        string `property:"name"`
	Description string `property:"description"`
	Model       string `property:"model"`
	Timezone    string `property:"timezone"`
	Version     string `property:"version"`

	Protocol protocolList

	Systems     map[string]bool
	Chassis     map[string]bool
	Mainchassis string
}

func NewBMCService(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) (*bmcService, error) {
	s := &bmcService{
		Systems: map[string]bool{},
		Chassis: map[string]bool{},
	}
	SetupBMCServiceEventStreams(ctx, s, ch, eb, ew)
	return s, nil
}

func SetupBMCServiceEventStreams(ctx context.Context, s *bmcService, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	// step 2: Add openbmc manager object after Managers collection has been created
	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.SelectEventResourceCreatedByURI("/redfish/v1/Managers"))
	if err != nil {
		fmt.Printf("Failed to create event stream processor: %s\n", err.Error())
		return
	}
	sp.RunOnce(func(event eh.Event) {
		s.AddOBMCManagerResource(ctx, ch)
	})
}

func (p protocolList) PluginType() domain.PluginType { return ProtocolPlugin }
func (p protocolList) RefreshProperty(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
	body interface{},
) {
	fmt.Printf("protocol dump: %s\n", meta)
	which, ok := meta["which"].(string)
	if !ok {
		fmt.Printf("\tbad which in meta: %s\n", meta)
		return
	}

	prot, ok := p[which]
	if !ok {
		fmt.Printf("\tbad which, no corresponding prot: %s\n", which)
		return
	}

	rrp.Value = map[string]interface{}{
		"ProtocolEnabled": prot.enabled,
		"Port":            prot.port,
	}

	for k, v := range prot.config {
		rrp.Value.(map[string]interface{})[k] = v
	}
}

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

	data, ok := meta["data"].(string)
	if data == "systems" {
		list := []map[string]string{}
		for k, _ := range s.Systems {
			list = append(list, map[string]string{"@odata.id": k})
		}
		rrp.Value = list
		return
	}

	if data == "chassis" {
		list := []map[string]string{}
		for k, _ := range s.Chassis {
			list = append(list, map[string]string{"@odata.id": k})
		}
		rrp.Value = list
		return
	}

	if data == "mainchassis" {
		if s.Mainchassis != "" {
			rrp.Value = map[string]string{"@odata.id": s.Mainchassis}
		} else {
			rrp.Value = map[string]string{}
		}
		return
	}

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

func (s *bmcService) AddSystem(uri string) {
	s.Lock()
	defer s.Unlock()
	fmt.Printf("DEBUG: ADDING SYSTEM(%s) to list: %s\n", uri, s.Systems)
	s.Systems[uri] = true
}

func (s *bmcService) RemoveSystem(uri string) {
	s.Lock()
	defer s.Unlock()
	fmt.Printf("DEBUG: REMOVING SYSTEM(%s) to list: %s\n", uri, s.Systems)
	delete(s.Systems, uri)
}

func (s *bmcService) AddChassis(uri string) {
	s.Lock()
	defer s.Unlock()
	if s.Mainchassis == "" {
		s.Mainchassis = uri
	}
	fmt.Printf("DEBUG: ADDING CHASSIS(%s) to list: %s\n", uri, s.Chassis)
	s.Chassis[uri] = true
}

func (s *bmcService) RemoveChassis(uri string) {
	s.Lock()
	defer s.Unlock()
	if s.Mainchassis == uri {
		s.Mainchassis = ""
	}
	fmt.Printf("DEBUG: REMOVING CHASSIS(%s) to list: %s\n", uri, s.Chassis)
	delete(s.Chassis, uri)
}

func (s *bmcService) AddOBMCManagerResource(ctx context.Context, ch eh.CommandHandler) {
	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			Collection:  false,
			ResourceURI: "/redfish/v1/Managers/bmc",
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
				"Id":                       "bmc",
				"Name@meta":                map[string]interface{}{"GET": map[string]interface{}{"plugin": "obmc_bmc", "property": "name"}},
				"ManagerType":              "BMC",
				"Description@meta":         map[string]interface{}{"GET": map[string]interface{}{"plugin": "obmc_bmc", "property": "description"}},
				"ServiceEntryPointUUID":    eh.NewUUID(),
				"UUID":                     eh.NewUUID(),
				"Model@meta":               map[string]interface{}{"GET": map[string]interface{}{"plugin": "obmc_bmc", "property": "model"}},
				"DateTime@meta":            map[string]interface{}{"GET": map[string]interface{}{"plugin": "datetime"}},
				"DateTimeLocalOffset@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": "obmc_bmc", "property": "timezone"}},
				"Status": map[string]interface{}{
					"State":  "Enabled",
					"Health": "OK",
				},
				"FirmwareVersion@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": "obmc_bmc", "property": "version"}},
				"NetworkProtocol":      map[string]interface{}{"@odata.id": "/redfish/v1/Managers/bmc/NetworkProtocol"},
				"EthernetInterfaces":   map[string]interface{}{"@odata.id": "/redfish/v1/Managers/bmc/EthernetInterfaces"},
				"Links": map[string]interface{}{
					"ManagerForServers@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": "obmc_bmc", "data": "systems"}},
					"ManagerForChassis@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": "obmc_bmc", "data": "chassis"}},
					// Leave this out for now
					//					"ManagerInChassis@meta":  map[string]interface{}{"GET": map[string]interface{}{"plugin": "obmc_bmc", "data": "mainchassis"}},
				},
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

	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			Collection:  false,
			ResourceURI: "/redfish/v1/Managers/bmc/NetworkProtocol",
			Type:        "#ManagerNetworkProtocol.v1_0_2.ManagerNetworkProtocol",
			Context:     "/redfish/v1/$metadata#ManagerNetworkProtocol.ManagerNetworkProtocol",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{"ConfigureManager"},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id":          "NetworkProtocol",
				"Name":        "Manager Network Protocol",
				"Description": "Manager Network Service Status",
				"Status": map[string]interface{}{
					"State":  "Enabled",
					"Health": "OK",
				},
				"HostName@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": "hostname"}},
				"FQDN":          "mymanager.mydomain.com",

				"HTTPS@meta":  map[string]interface{}{"GET": map[string]interface{}{"plugin": "protocol", "which": "https"}},
				"HTTP@meta":   map[string]interface{}{"GET": map[string]interface{}{"plugin": "protocol", "which": "http"}},
				"IPMI@meta":   map[string]interface{}{"GET": map[string]interface{}{"plugin": "protocol", "which": "ipmi"}},
				"SSH@meta":    map[string]interface{}{"GET": map[string]interface{}{"plugin": "protocol", "which": "ssh"}},
				"SNMP@meta":   map[string]interface{}{"GET": map[string]interface{}{"plugin": "protocol", "which": "snmp"}},
				"SSDP@meta":   map[string]interface{}{"GET": map[string]interface{}{"plugin": "protocol", "which": "ssdp"}},
				"Telnet@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": "protocol", "which": "telnet"}},
			}})

	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:         eh.NewUUID(),
			Collection: true,

			ResourceURI: "/redfish/v1/Managers/bmc/EthernetInterfaces",
			Type:        "#EthernetInterfaceCollection.EthernetInterfaceCollection",
			Context:     "/redfish/v1/$metadata#EthernetInterfaceCollection.EthernetInterfaceCollection",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // Read Only
				"PUT":    []string{}, // Read Only
				"PATCH":  []string{}, // Read Only
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name": "Ethernet Network Interface Collection",
			}})
}
