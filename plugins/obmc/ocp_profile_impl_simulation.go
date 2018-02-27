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
    plugins.OnURICreated(ctx, ew, "/redfish/v1/Managers", func(){ bmcSvc.AddResource(ctx, ch) })

    time.Sleep( 100 * time.Millisecond )
    prot, _ := NewNetProtocols(
        WithBMC(bmcSvc),
        WithProtocol("HTTPS", true, 443, nil),
        WithProtocol("HTTP", false, 80, nil),
        WithProtocol("IPMI", false, 623, nil),
        WithProtocol("SSH", false, 22, nil),
        WithProtocol("SNMP", false, 161, nil),
        WithProtocol("TELNET", false, 23, nil),
        WithProtocol("SSDP", false, 1900, map[string]interface{}{"NotifyMulticastIntervalSeconds": 600, "NotifyTTL": 5, "NotifyIPv6Scope": "Site"},),
        )
    prot.AddResource(ctx, ch)

	chas, _ := NewChassisService(ctx)
    plugins.OnURICreated(ctx, ew, "/redfish/v1/Chassis", func(){ chas.AddOBMCChassisResource(ctx, ch) })

	system, _ := NewSystemService(ctx)
    plugins.OnURICreated(ctx, ew, "/redfish/v1/Systems", func(){ system.AddOBMCSystemResource(ctx, ch) })

	domain.RegisterPlugin(func() domain.Plugin { return bmcSvc })
	domain.RegisterPlugin(func() domain.Plugin { return prot })
	domain.RegisterPlugin(func() domain.Plugin { return chas })
	domain.RegisterPlugin(func() domain.Plugin { return chas.thermalSensors })
	domain.RegisterPlugin(func() domain.Plugin { return system })

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
