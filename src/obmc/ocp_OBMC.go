// Build tags: only build this for the simulation build. Be sure to note the required blank line after.
// +build openbmc

package obmc

import (
	"context"
	"fmt"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	plugins "github.com/superchalupa/go-redfish/src/ocp"
	"github.com/superchalupa/go-redfish/src/ocp/bmc"
	"github.com/superchalupa/go-redfish/src/ocp/chassis"
	"github.com/superchalupa/go-redfish/src/ocp/protocol"
	"github.com/superchalupa/go-redfish/src/ocp/root"
	"github.com/superchalupa/go-redfish/src/ocp/session"
	"github.com/superchalupa/go-redfish/src/ocp/system"
	"github.com/superchalupa/go-redfish/src/ocp/thermal"
	"github.com/superchalupa/go-redfish/src/ocp/thermal/fans"
	"github.com/superchalupa/go-redfish/src/ocp/thermal/temperatures"
)

func InitOCP(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) *session.Service {
	// initial implementation is one BMC, one Chassis, and one System. If we
	// expand beyond that, we need to adjust stuff here.

	rootSvc, _ := root.New(
		plugins.UpdateProperty("test", "test property"),
	)

	sessionSvc, _ := session.New(
		session.Root(rootSvc),
	)

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
		chassis.AddManagedBy(bmcSvc),
		chassis.AddManagerInChassis(bmcSvc),
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

	bmcSvc.InChassis(chas)
	bmcSvc.AddManagerForChassis(chas)

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
		plugins.UpdateProperty("system_hostname", "CatfishHostname"),
	)

	bmcSvc.AddManagerForServer(system)
	chas.AddComputerSystem(system)

	therm, _ := thermal.New(
		thermal.InChassis(chas),
	)

	temps, _ := temperatures.New(
		temperatures.InThermal(therm),
	)

	fanObj, _ := fans.New(
		fans.InThermal(therm),
	)

	// Start background processing to update sensor data every 10 seconds
	go UpdateSensorList(ctx, temps)
	go UpdateFans(ctx, fanObj)

	// register all of the plugins (do this first so we dont get any race
	// conditions if somebody accesses the URIs before these plugins are
	// registered
	domain.RegisterPlugin(func() domain.Plugin { return rootSvc })
	domain.RegisterPlugin(func() domain.Plugin { return sessionSvc })
	domain.RegisterPlugin(func() domain.Plugin { return bmcSvc })
	domain.RegisterPlugin(func() domain.Plugin { return prot })
	domain.RegisterPlugin(func() domain.Plugin { return chas })
	domain.RegisterPlugin(func() domain.Plugin { return system })
	domain.RegisterPlugin(func() domain.Plugin { return therm })
	domain.RegisterPlugin(func() domain.Plugin { return temps })
	domain.RegisterPlugin(func() domain.Plugin { return fanObj })

	// and now add everything to the URI tree
	rootSvc.AddResource(ctx, ch, eb, ew)
	sessionSvc.AddResource(ctx, ch, eb, ew)
	bmcSvc.AddResource(ctx, ch, eb, ew)
	prot.AddResource(ctx, ch)
	chas.AddResource(ctx, ch)
	system.AddResource(ctx, ch, eb, ew)
	therm.AddResource(ctx, ch, eb, ew)
	temps.AddResource(ctx, ch, eb, ew)
	fanObj.AddResource(ctx, ch, eb, ew)

	bmcSvc.ApplyOption(plugins.UpdateProperty("manager.reset", func(event eh.Event, res *domain.HTTPCmdProcessedData) {
		BMCReset(ctx, event, res, eb)
	}))

	system.ApplyOption(plugins.UpdateProperty("computersystem.reset", func(event eh.Event, res *domain.HTTPCmdProcessedData) {
		fmt.Printf("Hello WORLD!\n\tGOT RESET EVENT\n")
		res.Results = map[string]interface{}{"RESET": "FAKE SIMULATED COMPUTER RESET"}
	}))

	return sessionSvc
}
