// Build tags: only build this for the simulation build. Be sure to note the required blank line after.
// +build simulation

package obmc

import (
	"context"
	"fmt"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	"github.com/superchalupa/go-redfish/plugins"
	ah "github.com/superchalupa/go-redfish/plugins/actionhandler"
	domain "github.com/superchalupa/go-redfish/redfishresource"
)

func init() {
	domain.RegisterInitFN(OCPProfileFactory)
}

func OCPProfileFactory(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {

	// initial implementation is one BMC, one Chassis, and one System. If we
	// expand beyond that, we need to adjust stuff here.

	bmcSvc, _ := NewBMCService()
	bmcSvc.URIName = "OBMC"
	bmcSvc.Name = "OBMC simulation"
	bmcSvc.Description = "The most open source BMC ever."
	bmcSvc.Model = "Michaels RAD BMC"
	bmcSvc.Timezone = "-05:00"
	bmcSvc.Version = "1.0.0"

/*
	bmcSvc.Protocol = protocolList{
		"https":  protocol{enabled: true, port: 443},
		"http":   protocol{enabled: false, port: 80},
		"ipmi":   protocol{enabled: false, port: 623},
		"ssh":    protocol{enabled: false, port: 22},
		"snmp":   protocol{enabled: false, port: 161},
		"telnet": protocol{enabled: false, port: 23},
		"ssdp": protocol{enabled: false, port: 1900,
			config: map[string]interface{}{"NotifyMulticastIntervalSeconds": 600, "NotifyTTL": 5, "NotifyIPv6Scope": "Site"},
		}}
*/

    plugins.OnURICreated(ctx, ew, "/redfish/v1/Managers", func(){ bmcSvc.AddOBMCManagerResource(ctx, ch) })

	chas, _ := NewChassisService(ctx)
	CreateChassisStreamProcessors(ctx, chas, ch, eb, ew)

	system, _ := NewSystemService(ctx)
	CreateSystemStreamProcessors(ctx, system, ch, eb, ew)

	domain.RegisterPlugin(func() domain.Plugin { return bmcSvc })
	//domain.RegisterPlugin(func() domain.Plugin { return bmcSvc.Protocol })
	domain.RegisterPlugin(func() domain.Plugin { return chas })
	domain.RegisterPlugin(func() domain.Plugin { return chas.thermalSensors })
	domain.RegisterPlugin(func() domain.Plugin { return system })

	// example:
	// go ret.runbackgroundstuff(ctx)

	// stream processor for action events
	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.CustomFilter(ah.SelectAction("/redfish/v1/Managers/bmc/Actions/Manager.Reset")))
	if err != nil {
		fmt.Printf("Failed to create event stream processor: %s\n", err.Error())
		return // todo: tear down all the prior event stream processors, too
	}
	sp.RunForever(func(event eh.Event) {
		fmt.Printf("GOT ACTION EVENT!!!\n")

		eb.HandleEvent(ctx, eh.NewEvent(domain.HTTPCmdProcessed, domain.HTTPCmdProcessedData{
			CommandID:  event.Data().(ah.GenericActionEventData).CmdID,
			Results:    map[string]interface{}{"RESET": "FAKE SIMULATED RESET"},
			StatusCode: 200,
			Headers:    map[string]string{},
		}, time.Now()))
	})
}
