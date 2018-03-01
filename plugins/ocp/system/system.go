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

// OCP Profile Redfish System object
type service struct {
	*plugins.Service
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
				"Id":           "2M220100SL",
				"Name@meta":    map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": "name"}},
				"SystemType":   "Physical",
				"AssetTag":     "CATFISHASSETTAG",
				"Manufacturer": "CatfishManufacturer",
				"Model":        "YellowCat1000",
				"SerialNumber": "2M220100SL",
				"SKU":          "",
				"PartNumber":   "",
				"Description":  "Catfish Implementation Recipe of simple scale-out monolithic server",
				"UUID":         "00000000-0000-0000-0000-000000000000",
				"HostName":     "catfishHostname",
				"PowerState":   "On",
				"BiosVersion":  "X00.1.2.3.4(build-23)",
				"Status": map[string]interface{}{
					"State":  "Enabled",
					"Health": "OK",
				},
				"IndicatorLED": "Off",
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
				"LogServices": map[string]interface{}{
					"@odata.id": "/redfish/v1/Systems/2M220100SL/LogServices",
				},
				"Links": map[string]interface{}{
					"Chassis": []map[string]interface{}{
						map[string]interface{}{
							"@odata.id": "/redfish/v1/Chassis/A33",
						},
					},
					"ManagedBy": []map[string]interface{}{
						map[string]interface{}{
							"@odata.id": "/redfish/v1/Managers/bmc",
						},
					},
					"Oem": map[string]interface{}{},
				},
				"Actions": map[string]interface{}{
					"#ComputerSystem.Reset": map[string]interface{}{
						"target": "/redfish/v1/Systems/2M220100SL/Actions/ComputerSystem.Reset",
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
