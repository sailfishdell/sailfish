package obmc

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/superchalupa/go-redfish/plugins"
	domain "github.com/superchalupa/go-redfish/redfishresource"

	"github.com/godbus/dbus"
	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	ah "github.com/superchalupa/go-redfish/plugins/actionhandler"
)

var (
	BmcPlugin      = domain.PluginType("obmc_bmc")
	ProtocolPlugin = domain.PluginType("protocol")

	DbusTimeout time.Duration = 1
)

func init() {
	domain.RegisterInitFN(InitService)
}

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

	protocol protocolList

	systems     map[string]bool
	chassis     map[string]bool
	mainchassis string
}

func NewBMCService(ctx context.Context) (*bmcService, error) {
	return &bmcService{
		Name:        "OBMC",
		Description: "The most open source BMC ever.",
		Model:       "Michaels RAD BMC",
		Timezone:    "-05:00",
		Version:     "1.0.0",
		systems:     map[string]bool{},
		chassis:     map[string]bool{},
		protocol: protocolList{
			"https":  protocol{enabled: true, port: 443},
			"http":   protocol{enabled: false, port: 80},
			"ipmi":   protocol{enabled: false, port: 623},
			"ssh":    protocol{enabled: false, port: 22},
			"snmp":   protocol{enabled: false, port: 161},
			"telnet": protocol{enabled: false, port: 23},
			"ssdp": protocol{enabled: false, port: 1900,
				config: map[string]interface{}{"NotifyMulticastIntervalSeconds": 600, "NotifyTTL": 5, "NotifyIPv6Scope": "Site"},
			},
		}}, nil
}

// wait in a listener for the root service to be created, then extend it
func InitService(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	// step 1: Is this an actual openbmc?
	// TODO: add test here

	s, err := NewBMCService(ctx)
	if err != nil {
		return
	}
	SetupBMCServiceEventStreams(ctx, s, ch, eb, ew)
	SetupBMCServiceDbusConnections(ctx, s, ch, eb, ew)

	// Singleton for bmc plugin: we can pull data out of ourselves on GET/etc.
	// after this point, the bmc object we just created is "live"
	domain.RegisterPlugin(func() domain.Plugin { return s })
	domain.RegisterPlugin(func() domain.Plugin { return s.protocol })

	// initial implementation is one BMC, one Chassis, and one System. If we
	// expand beyond that, we need to adjust stuff here.
	chas, err := NewChassisService(ctx)
	if err != nil {
		return
	}
	InitChassisService(ctx, chas, ch, eb, ew)

	system, err := NewSystemService(ctx)
	if err != nil {
		return
	}
	InitSystemService(ctx, system, ch, eb, ew)
}

func SetupBMCServiceDbusConnections(ctx context.Context, s *bmcService, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	/*    conn, err := dbus.SystemBus()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot register dbus_property plugin, could not connect to System Bus: %s\n", err.Error())
			return
		}
	    bus := "xyz.openbmc_project.Software.BMC.Updater"
	    path :=  "/xyz/openbmc_project/software/13264da3"
	    intfc := "xyz.openbmc_project.Software.Version"
	    prop  := "Version"
		busObject := t.conn.Object(bus, dbus.ObjectPath(path))
		variant, err := busObject.GetProperty(intfc + "." + prop)
	*/
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

	// we have a semi-collection of links ot systems and chassis we maintain, so add a event stream processor to keep those updated
	sp, err = plugins.NewEventStreamProcessor(ctx, ew, plugins.SelectEventResourceCreatedByURIPrefix("/redfish/v1/Systems/"))
	if err != nil {
		fmt.Printf("Failed to create event stream processor: %s\n", err.Error())
		return // todo: tear down all the prior event stream processors, too
	}
	sp.RunForever(func(event eh.Event) {
		if data, ok := event.Data().(domain.RedfishResourceCreatedData); ok {
			s.AddSystem(data.ResourceURI)
		}
	})

	sp, err = plugins.NewEventStreamProcessor(ctx, ew, plugins.SelectEventResourceCreatedByURIPrefix("/redfish/v1/Chassis/"))
	if err != nil {
		fmt.Printf("Failed to create event stream processor: %s\n", err.Error())
		return // todo: tear down all the prior event stream processors, too
	}
	sp.RunForever(func(event eh.Event) {
		if data, ok := event.Data().(domain.RedfishResourceCreatedData); ok {
			s.AddChassis(data.ResourceURI)
		}
	})

	sp, err = plugins.NewEventStreamProcessor(ctx, ew, plugins.SelectEventResourceRemovedByURIPrefix("/redfish/v1/Systems/"))
	if err != nil {
		fmt.Printf("Failed to create event stream processor: %s\n", err.Error())
		return // todo: tear down all the prior event stream processors, too
	}
	sp.RunForever(func(event eh.Event) {
		if data, ok := event.Data().(domain.RedfishResourceRemovedData); ok {
			s.RemoveSystem(data.ResourceURI)
		}
	})

	sp, err = plugins.NewEventStreamProcessor(ctx, ew, plugins.SelectEventResourceRemovedByURIPrefix("/redfish/v1/Chassis/"))
	if err != nil {
		fmt.Printf("Failed to create event stream processor: %s\n", err.Error())
		return // todo: tear down all the prior event stream processors, too
	}
	sp.RunForever(func(event eh.Event) {
		if data, ok := event.Data().(domain.RedfishResourceRemovedData); ok {
			s.RemoveChassis(data.ResourceURI)
		}
	})

	// stream processor for action events
	sp, err = plugins.NewEventStreamProcessor(ctx, ew, plugins.CustomFilter(ah.SelectAction("/redfish/v1/Managers/bmc/Actions/Manager.Reset")))
	if err != nil {
		fmt.Printf("Failed to create event stream processor: %s\n", err.Error())
		return // todo: tear down all the prior event stream processors, too
	}
	sp.RunForever(func(event eh.Event) {
		bus := "org.openbmc.control.Bmc"
		path := "/org/openbmc/control/bmc0"
		intfc := "org.openbmc.control.Bmc"

		fmt.Printf("connect to system bus\n")
		conn, err := dbus.SystemBus()
		statusCode := 200
		statusMessage := "OK"
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot connect to System Bus: %s\n", err.Error())
			statusCode = 501
			statusMessage = "ERROR: Cannot attach to dbus system bus"
		}
		fmt.Printf("connect to %s path %s\n", bus, path)
		busObject := conn.Object(bus, dbus.ObjectPath(path))

		fmt.Printf("parse resetType: %s\n", event.Data())
		ad := event.Data().(ah.GenericActionEventData)
		resetType, _ := ad.ActionData.(map[string]interface{})["ResetType"]
		call := "undefined"
		if resetType == "ForceRestart" {
			call = "coldReset"
		}
		if resetType == "GracefulRestart" {
			call = "warmReset"
		}
		fmt.Printf("\tgot: %s\n", resetType)

		// no way to cancel this based on context cancellation form original request.

		fmt.Printf("make call\n")
		callObj := busObject.Call(intfc+"."+call, 0)
		select {
		case <-callObj.Done:
			fmt.Printf("donecall. err: %s\n", callObj.Err.Error())
		case <-ctx.Done():
			statusCode = 501
			statusMessage = "Context cancelled."
			// give up
		case <-time.After(time.Duration(DbusTimeout) * time.Second):
			statusCode = 501
			statusMessage = "ERROR: Dbus call timed out"
			// give up
		}

		eb.HandleEvent(ctx, eh.NewEvent(domain.HTTPCmdProcessed, domain.HTTPCmdProcessedData{
			CommandID:  event.Data().(ah.GenericActionEventData).CmdID,
			Results:    map[string]interface{}{"RESET": statusMessage},
			StatusCode: statusCode,
			Headers:    map[string]string{},
		}, time.Now()))
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
		for k, _ := range s.systems {
			list = append(list, map[string]string{"@odata.id": k})
		}
		rrp.Value = list
		return
	}

	if data == "chassis" {
		list := []map[string]string{}
		for k, _ := range s.chassis {
			list = append(list, map[string]string{"@odata.id": k})
		}
		rrp.Value = list
		return
	}

	if data == "mainchassis" {
		if s.mainchassis != "" {
			rrp.Value = map[string]string{"@odata.id": s.mainchassis}
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
	fmt.Printf("DEBUG: ADDING SYSTEM(%s) to list: %s\n", uri, s.systems)
	s.systems[uri] = true
}

func (s *bmcService) RemoveSystem(uri string) {
	s.Lock()
	defer s.Unlock()
	fmt.Printf("DEBUG: REMOVING SYSTEM(%s) to list: %s\n", uri, s.systems)
	delete(s.systems, uri)
}

func (s *bmcService) AddChassis(uri string) {
	s.Lock()
	defer s.Unlock()
	if s.mainchassis == "" {
		s.mainchassis = uri
	}
	fmt.Printf("DEBUG: ADDING CHASSIS(%s) to list: %s\n", uri, s.chassis)
	s.chassis[uri] = true
}

func (s *bmcService) RemoveChassis(uri string) {
	s.Lock()
	defer s.Unlock()
	if s.mainchassis == uri {
		s.mainchassis = ""
	}
	fmt.Printf("DEBUG: REMOVING CHASSIS(%s) to list: %s\n", uri, s.chassis)
	delete(s.chassis, uri)
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
