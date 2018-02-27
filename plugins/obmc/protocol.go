package obmc

import (
    "context"

	domain "github.com/superchalupa/go-redfish/redfishresource"
	"github.com/superchalupa/go-redfish/plugins"
	eh "github.com/looplab/eventhorizon"
)

const (
	ProtocolPlugin = domain.PluginType("protocol")
)

type protocol struct {
	enabled bool
	port    int
	config  map[string]interface{}
}

type netProtocols struct {
    *plugins.Service
	bmc *bmcService

    protocols map[string]protocol
}

type ProtocolOption func(*netProtocols) error

func NewNetProtocols(options... ProtocolOption) (*netProtocols, error) {
	p := &netProtocols{
        Service: plugins.NewService(plugins.PluginType(ProtocolPlugin)),
        protocols: map[string]protocol{},
        }
    p.ApplyOption(options...)
    return p, nil
}

func (p *netProtocols) ApplyOption(options ...ProtocolOption) error {
    for _, o := range options {
        err := o(p)
        if err != nil {
            return err
        }
    }
    return nil
}

func WithBMC(b *bmcService) ProtocolOption {
    return func(p *netProtocols) error {
        p.bmc = b
        return nil
    }
}

func WithProtocol(name string, enabled bool, port int, config map[string]interface{}) ProtocolOption {
    return func(p *netProtocols) error {
        newP := protocol{enabled: enabled, port: port}
        newP.config = map[string]interface{}{}
        for k, v := range config {
            newP.config[k] = v
        }
        p.protocols[name] = newP
        return nil
    }
}

func (p *netProtocols) GetOdataID() string { return p.bmc.GetOdataID() + "/NetworkProtocol" }
func (p *netProtocols) RefreshProperty(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
	body interface{},
) {
    for k, v := range p.protocols {
        tempProto := map[string]interface{}{}
        tempProto["ProtocolEnabled"] = v.enabled
        tempProto["Port"] = v.port
        for configK, configV := range v.config {
            tempProto[configK] = configV
        }
        rrp.Value.(map[string]interface{})[k] = tempProto
    }
}

func (s *netProtocols) AddResource(ctx context.Context, ch eh.CommandHandler) {
	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			Collection:  false,
			ResourceURI: s.GetOdataID(),
			Type:        "#ManagerNetworkProtocol.v1_0_2.ManagerNetworkProtocol",
			Context:     "/redfish/v1/$metadata#ManagerNetworkProtocol.ManagerNetworkProtocol",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{"ConfigureManager"},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
            Meta: map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()) }},
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
			}})


	ch.HandleCommand(ctx,
		&domain.UpdateRedfishResourceProperties{
			ID: s.bmc.GetUUID(),
			Properties: map[string]interface{}{
				"NetworkProtocol":      map[string]interface{}{"@odata.id": s.GetOdataID() },
			},
		})
}



/*
				"EthernetInterfaces":   map[string]interface{}{"@odata.id": "/redfish/v1/Managers/bmc/EthernetInterfaces"},

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

