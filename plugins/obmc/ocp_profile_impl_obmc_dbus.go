// Build tags: only build this for the simulation build. Be sure to note the required blank line after.
// +build openbmc

package obmc

import (
	"context"
	"fmt"
	"time"
    "os"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	domain "github.com/superchalupa/go-redfish/redfishresource"
	ah "github.com/superchalupa/go-redfish/plugins/actionhandler"
	"github.com/superchalupa/go-redfish/plugins"
	"github.com/godbus/dbus"
	mydbus "github.com/superchalupa/go-redfish/plugins/dbus"
)

func init() {
	domain.RegisterInitFN(InitService)
}

const (
	DbusTimeout time.Duration = 1
)

type simulationImplBmc struct {
	*bmcService
}

func InitService(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	// initial implementation is one BMC, one Chassis, and one System. If we
	// expand beyond that, we need to adjust stuff here.

	bmcSvc, _ := NewBMCService(ctx, ch, eb, ew)
	bmcSvc.Name = "OBMC simulation"
	bmcSvc.Description = "The most open source BMC ever."
	bmcSvc.Model = "Michaels RAD BMC"
	bmcSvc.Timezone = "-05:00"
	bmcSvc.Version = "1.0.0"
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

	chas, _ := NewChassisService(ctx)
	CreateChassisStreamProcessors(ctx, chas, ch, eb, ew)

	system, _ := NewSystemService(ctx)
	CreateSystemStreamProcessors(ctx, system, ch, eb, ew)


	domain.RegisterPlugin(func() domain.Plugin { return bmcSvc })
	domain.RegisterPlugin(func() domain.Plugin { return bmcSvc.Protocol })
    domain.RegisterPlugin(func() domain.Plugin { return chas })
    domain.RegisterPlugin(func() domain.Plugin { return chas.thermalSensors })
    domain.RegisterPlugin(func() domain.Plugin { return system })

	// go ret.runbackgroundstuff(ctx)

	// stream processor for action events
	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.CustomFilter(ah.SelectAction("/redfish/v1/Managers/bmc/Actions/Manager.Reset")))
	if err != nil {
		fmt.Printf("Failed to create event stream processor: %s\n", err.Error())
		return // todo: tear down all the prior event stream processors, too
	}
	sp.RunForever(func(event eh.Event) {
		bus := "org.openbmc.control.Bmc"
		path := "/org/openbmc/control/bmc0"
		intfc := "org.openbmc.control.Bmc"

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
		if call == "undefined" {
			return
		}

		fmt.Printf("connect to system bus\n")
		statusCode := 200
		statusMessage := "OK"

		conn, err := dbus.SystemBus()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot connect to System Bus: %s\n", err.Error())
			statusCode = 501
			statusMessage = "ERROR: Cannot attach to dbus system bus"
		}

		dh := mydbus.NewDbusHelper(conn, bus, path)
		timedctx, cancel := context.WithTimeout(ctx, time.Duration(DbusTimeout))
		defer cancel()
		_, err = dh.DbusCall(timedctx, 0, intfc+"."+call)
		if err != nil {
			statusCode = 501
			statusMessage = "Internal call failed"
		}

		eb.HandleEvent(ctx, eh.NewEvent(domain.HTTPCmdProcessed, domain.HTTPCmdProcessedData{
			CommandID:  event.Data().(ah.GenericActionEventData).CmdID,
			Results:    map[string]interface{}{"RESET": statusMessage},
			StatusCode: statusCode,
			Headers:    map[string]string{},
		}, time.Now()))
	})
}
