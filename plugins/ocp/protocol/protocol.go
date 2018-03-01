package protocol

import (
	"context"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/plugins"
	domain "github.com/superchalupa/go-redfish/redfishresource"
)

const (
	ProtocolPlugin = domain.PluginType("protocol")
)

type protocol struct {
	enabled bool
	port    int
	config  map[string]interface{}
}

type bmcInt interface {
	GetOdataID() string
	GetUUID() eh.UUID
}

type service struct {
	*plugins.Service
	bmc bmcInt

	protocols map[string]protocol
}

func New(options ...interface{}) (*service, error) {
	p := &service{
		Service:   plugins.NewService(plugins.PluginType(ProtocolPlugin)),
		protocols: map[string]protocol{},
	}
	p.ApplyOption(options...)
	return p, nil
}

func WithBMC(b bmcInt) Option {
	return func(p *service) error {
		p.bmc = b
		return nil
	}
}

func WithProtocol(name string, enabled bool, port int, config map[string]interface{}) Option {
	return func(p *service) error {
		newP := protocol{enabled: enabled, port: port}
		newP.config = map[string]interface{}{}
		for k, v := range config {
			newP.config[k] = v
		}
		p.protocols[name] = newP
		return nil
	}
}

func (p *service) GetOdataID() string { return p.bmc.GetOdataID() + "/NetworkProtocol" }
func (p *service) RefreshProperty(
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

func (s *service) AddResource(ctx context.Context, ch eh.CommandHandler) {
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
			Meta: map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType())}},
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
				"NetworkProtocol": map[string]interface{}{"@odata.id": s.GetOdataID()},
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
