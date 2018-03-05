package system

import (
	"context"
	"fmt"
	"time"

	"github.com/superchalupa/go-redfish/plugins"
	domain "github.com/superchalupa/go-redfish/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	ah "github.com/superchalupa/go-redfish/plugins/actionhandler"
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

func (s *service) AddResource(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
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
				"IndicatorLED@meta": s.MetaReadOnlyProperty("led"),
				"HostName@meta":     s.MetaReadOnlyProperty("system_hostname"),

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

	// The following redfish resource is created only for the purpose of being
	// a 'receiver' for the action command specified above.
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			ResourceURI: s.GetOdataID() + "/Actions/ComputerSystem.Reset",
			Type:        "Action",
			Context:     "Action",
			Plugin:      "GenericActionHandler",
			Privileges: map[string]interface{}{
				"POST": []string{"ConfigureComponents"},
			},
			Properties: map[string]interface{}{},
		},
	)

	// stream processor for action events
	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.CustomFilter(ah.SelectAction(s.GetOdataID()+"/Actions/ComputerSystem.Reset")))
	if err != nil {
		fmt.Printf("Failed to create event stream processor: %s\n", err.Error())
		return
	}
	sp.RunForever(func(event eh.Event) {
		fmt.Printf("GOT ACTION EVENT!!!\n")

		eventData := domain.HTTPCmdProcessedData{
			CommandID:  event.Data().(ah.GenericActionEventData).CmdID,
			Results:    map[string]interface{}{"msg": "Not Implemented"},
			StatusCode: 500,
			Headers:    map[string]string{},
		}

		handler := s.GetProperty("computersystem.reset")
		if handler != nil {
			if fn, ok := handler.(func(eh.Event, *domain.HTTPCmdProcessedData)); ok {
				fn(event, &eventData)
			}
		}

		responseEvent := eh.NewEvent(domain.HTTPCmdProcessed, eventData, time.Now())
		eb.HandleEvent(ctx, responseEvent)
	})
}
