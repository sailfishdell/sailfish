package system

import (
	"context"

	"github.com/superchalupa/go-redfish/plugins"
	domain "github.com/superchalupa/go-redfish/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

var (
	OBMC_SystemPlugin = domain.PluginType("obmc_system")
)

type odataInt interface {
	GetOdataID() string
}

// OCP Profile Redfish System object
type service struct {
	*plugins.Service

	bmc  odataInt
	chas odataInt
}

func New(options ...interface{}) (*service, error) {
	s := &service{
		Service: plugins.NewService(plugins.PluginType(OBMC_SystemPlugin)),
	}
	s.ApplyOption(plugins.UUID())
	s.ApplyOption(options...)
	s.ApplyOption(plugins.PropertyOnce("uri", "/redfish/v1/Systems/"+s.GetProperty("unique_name").(string)))
	return s, nil
}

func WithUniqueName(uri string) plugins.Option {
	return plugins.PropertyOnce("unique_name", uri)
}

func ManagedBy(b odataInt) Option {
	return func(p *service) error {
		p.bmc = b
		return nil
	}
}

func InChassis(b odataInt) Option {
	return func(p *service) error {
		p.chas = b
		return nil
	}
}

func (s *service) AddResource(ctx context.Context, ch eh.CommandHandler) {
	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          s.GetUUID(),
			Collection:  false,
			ResourceURI: s.GetOdataID(),
			Type:        "#ComputerSystem.v1_1_0.ComputerSystem",
			Context:     "/redfish/v1/$metadata#ComputerSystem.ComputerSystem",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{"ConfigureManager"},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id":                s.GetProperty("unique_name"),
				"Name@meta":         s.MetaReadOnlyProperty("name"),
				"SystemType@meta":   s.MetaReadOnlyProperty("system_type"),
				"AssetTag@meta":     s.MetaReadOnlyProperty("asset_tag"),
				"Manufacturer@meta": s.MetaReadOnlyProperty("manufacturer"),
				"Model@meta":        s.MetaReadOnlyProperty("model"),
				"SerialNumber@meta": s.MetaReadOnlyProperty("serial_number"),
				"SKU@meta":          s.MetaReadOnlyProperty("sku"),
				"PartNumber@meta":   s.MetaReadOnlyProperty("part_number"),
				"Description@meta":  s.MetaReadOnlyProperty("description"),
				"PowerState@meta":   s.MetaReadOnlyProperty("power_state"),
				"BiosVersion@meta":  s.MetaReadOnlyProperty("bios_version"),
				"IndicatorLED":      s.MetaReadOnlyProperty("led"),

				"HostName": map[string]interface{}{"GET": map[string]interface{}{"plugin": "hostname"}},

				"Links": map[string]interface{}{
					"Chassis": []map[string]interface{}{
						map[string]interface{}{
							"@odata.id": s.chas.GetOdataID(),
						},
					},
					"ManagedBy": []map[string]interface{}{
						map[string]interface{}{
							"@odata.id": s.bmc.GetOdataID(),
						},
					},
					"Oem": map[string]interface{}{},
				},

				"Status": map[string]interface{}{
					"State":  "Enabled",
					"Health": "OK",
				},
				"Boot": map[string]interface{}{
					"BootSourceOverrideEnabled":    "Once",
					"BootSourceOverrideMode":       "UEFI",
					"UefiTargetBootSourceOverride": "uefiDevicePath",
					"BootSourceOverrideTarget":     "Pxe",
					"BootSourceOverrideTarget@Redfish.AllowableValues": []string{
						"None",
						"Pxe",
						"Usb",
						"Hdd",
						"BiosSetup",
						"UefiTarget",
						"UefiHttp",
					},
				},

				"Actions": map[string]interface{}{
					"#ComputerSystem.Reset": map[string]interface{}{
						"target": s.GetOdataID() + "/Actions/ComputerSystem.Reset",
						"ResetType@Redfish.AllowableValues": []string{
							"On",
							"ForceOff",
							"GracefulShutdown",
							"ForceRestart",
							"Nmi",
							"GracefulRestart",
							"ForceOn",
						},
					},
				},
			}})
}

/*
TODO:
				"LogServices": map[string]interface{}{
					"@odata.id":  s.GetOdataID() + "/LogServices",
				},

*/
