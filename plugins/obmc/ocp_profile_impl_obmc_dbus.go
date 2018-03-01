// Build tags: only build this for the simulation build. Be sure to note the required blank line after.
// +build openbmc

package obmc

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/godbus/dbus"
	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	"github.com/superchalupa/go-redfish/plugins"
	ah "github.com/superchalupa/go-redfish/plugins/actionhandler"
	mydbus "github.com/superchalupa/go-redfish/plugins/dbus"
	domain "github.com/superchalupa/go-redfish/redfishresource"

	"github.com/superchalupa/go-redfish/plugins/ocp/bmc"
	"github.com/superchalupa/go-redfish/plugins/ocp/chassis"
	"github.com/superchalupa/go-redfish/plugins/ocp/protocol"
	"github.com/superchalupa/go-redfish/plugins/ocp/system"
)

func init() {
	domain.RegisterInitFN(OCPProfileFactory)
}

const (
	DbusTimeout time.Duration = 1
)

func OCPProfileFactory(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	// initial implementation is one BMC, one Chassis, and one System. If we
	// expand beyond that, we need to adjust stuff here.

	bmcSvc, _ := bmc.New(
		bmc.WithUniqueName("OBMC"),
		plugins.UpdateProperty("name", "OBMC Simulation"),
		plugins.UpdateProperty("description", "The most open source BMC ever."),
		plugins.UpdateProperty("model", "Michaels RAD BMC"),
		plugins.UpdateProperty("timezone", "-05:00"),
		plugins.UpdateProperty("version", "1.0.0"),
	)

	prot, _ := protocol.New(
		protocol.WithBMC(bmcSvc),
		protocol.WithProtocol("HTTPS", true, 443, nil),
		protocol.WithProtocol("HTTP", false, 80, nil),
		protocol.WithProtocol("IPMI", false, 623, nil),
		protocol.WithProtocol("SSH", false, 22, nil),
		protocol.WithProtocol("SNMP", false, 161, nil),
		protocol.WithProtocol("TELNET", false, 23, nil),
		protocol.WithProtocol("SSDP", false, 1900,
			map[string]interface{}{"NotifyMulticastIntervalSeconds": 600, "NotifyTTL": 5, "NotifyIPv6Scope": "Site"}),
	)

	chas, _ := chassis.New(
		chassis.ManagedBy(bmcSvc),
		chassis.WithUniqueName("1"),
		plugins.UpdateProperty("name", "Catfish System Chassis"),
		plugins.UpdateProperty("chassis_type", "RackMount"),
		plugins.UpdateProperty("model", "YellowCat1000"),
		plugins.UpdateProperty("serial_number", "2M220100SL"),
		plugins.UpdateProperty("sku", "The SKU"),
		plugins.UpdateProperty("part_number", "Part2468"),
		plugins.UpdateProperty("asset_tag", "CATFISHASSETTAG"),
		plugins.UpdateProperty("chassis_type", "RackMount"),
		plugins.UpdateProperty("manufacturer", "Cat manufacturer"),
	)

	system, _ := system.New(
		system.WithUniqueName("1"),
		system.ManagedBy(bmcSvc),
		system.InChassis(chas),
		plugins.UpdateProperty("name", "Catfish System"),
		plugins.UpdateProperty("system_type", "Physical"),
		plugins.UpdateProperty("asset_tag", "CATFISHASSETTAG"),
		plugins.UpdateProperty("manufacturer", "Cat manufacturer"),
		plugins.UpdateProperty("model", "YellowCat1000"),
		plugins.UpdateProperty("serial_number", "2M220100SL"),
		plugins.UpdateProperty("sku", "The SKU"),
		plugins.UpdateProperty("part_number", "Part2468"),
		plugins.UpdateProperty("description", "Catfish Implementation Recipe of simple scale-out monolithic server"),
		plugins.UpdateProperty("power_state", "On"),
		plugins.UpdateProperty("bios_version", "X00.1.2.3.4(build-23)"),
		plugins.UpdateProperty("led", "On"),
	)

	// register all of the plugins (do this first so we dont get any race
	// conditions if somebody accesses the URIs before these plugins are
	// registered
	domain.RegisterPlugin(func() domain.Plugin { return bmcSvc })
	domain.RegisterPlugin(func() domain.Plugin { return prot })
	domain.RegisterPlugin(func() domain.Plugin { return chas })
	//domain.RegisterPlugin(func() domain.Plugin { return chas.thermalSensors })
	domain.RegisterPlugin(func() domain.Plugin { return system })

	// and now add everything to the URI tree
	time.Sleep(250 * time.Millisecond)
	bmcSvc.AddResource(ctx, ch, eb, ew)
	time.Sleep(250 * time.Millisecond)
	prot.AddResource(ctx, ch)
	chas.AddResource(ctx, ch)
	system.AddResource(ctx, ch)

	bmcSvc.ApplyOption(plugins.UpdateProperty("manager.reset", func(event eh.Event, res *domain.HTTPCmdProcessedData) {
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
	}))
}
