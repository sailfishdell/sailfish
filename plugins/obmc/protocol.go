package obmc

import (
    "fmt"
    "context"

	domain "github.com/superchalupa/go-redfish/redfishresource"
)

const (
	ProtocolPlugin = domain.PluginType("protocol")
)

type protocolList map[string]protocol
type protocol struct {
	enabled bool
	port    int
	config  map[string]interface{}
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


/*
				"NetworkProtocol":      map[string]interface{}{"@odata.id": "/redfish/v1/Managers/bmc/NetworkProtocol"},
				"EthernetInterfaces":   map[string]interface{}{"@odata.id": "/redfish/v1/Managers/bmc/EthernetInterfaces"},

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

				"HTTPS@meta":  map[string]interface{}{"GET": map[string]interface{}{"plugin": ProtocolPlugin, "which": "https"}},
				"HTTP@meta":   map[string]interface{}{"GET": map[string]interface{}{"plugin": ProtocolPlugin, "which": "http"}},
				"IPMI@meta":   map[string]interface{}{"GET": map[string]interface{}{"plugin": ProtocolPlugin, "which": "ipmi"}},
				"SSH@meta":    map[string]interface{}{"GET": map[string]interface{}{"plugin": ProtocolPlugin, "which": "ssh"}},
				"SNMP@meta":   map[string]interface{}{"GET": map[string]interface{}{"plugin": ProtocolPlugin, "which": "snmp"}},
				"SSDP@meta":   map[string]interface{}{"GET": map[string]interface{}{"plugin": ProtocolPlugin, "which": "ssdp"}},
				"Telnet@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": ProtocolPlugin, "which": "telnet"}},
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


*/

