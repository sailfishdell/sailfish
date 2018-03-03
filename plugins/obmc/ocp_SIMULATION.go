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
	domain "github.com/superchalupa/go-redfish/redfishresource"

	"github.com/superchalupa/go-redfish/plugins/ocp/bmc"
	"github.com/superchalupa/go-redfish/plugins/ocp/chassis"
	"github.com/superchalupa/go-redfish/plugins/ocp/protocol"
	"github.com/superchalupa/go-redfish/plugins/ocp/system"
	"github.com/superchalupa/go-redfish/plugins/ocp/thermal"
	"github.com/superchalupa/go-redfish/plugins/ocp/thermal/fans"
	"github.com/superchalupa/go-redfish/plugins/ocp/thermal/temperatures"
)

func init() {
	domain.RegisterInitFN(OCPProfileFactory)
}

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

	therm, _ := thermal.New(
		thermal.InChassis(chas),
	)

	temps, _ := temperatures.New(
		temperatures.InThermal(therm),
		temperatures.WithSensor("inlet",
			&temperatures.RedfishThermalSensor{
				Name:                      "inlet temp sensor",
				SensorNumber:              0,
				ReadingCelsius:            22,
				UpperThresholdNonCritical: 100,
				UpperThresholdCritical:    150,
				UpperThresholdFatal:       200,
				MinReadingRangeTemp:       0,
				MaxReadingRangeTemp:       250,
				PhysicalContext:           "inlet",
			},
		),
		temperatures.WithSensor("ndc",
			&temperatures.RedfishThermalSensor{
				Name:                      "ndc temp sensor",
				SensorNumber:              1,
				ReadingCelsius:            23,
				UpperThresholdNonCritical: 100,
				UpperThresholdCritical:    150,
				UpperThresholdFatal:       200,
				MinReadingRangeTemp:       0,
				MaxReadingRangeTemp:       250,
				PhysicalContext:           "ndc",
			},
		),
	)

	fans, _ := fans.New(
		fans.InThermal(therm),
		fans.WithSensor("fan_X",
			&fans.RedfishFan{
				Name:            "inlet fan",
				PhysicalContext: "inlet",

				Reading:      2500,
				ReadingUnits: "RPM",

				UpperThresholdNonCritical: 3500,
				UpperThresholdCritical:    3600,
				UpperThresholdFatal:       3700,

				LowerThresholdNonCritical: 1000,
				LowerThresholdCritical:    900,
				LowerThresholdFatal:       800,

				MinReadingRange: 500,
				MaxReadingRange: 5000,
			},
		),
	)

	// register all of the plugins (do this first so we dont get any race
	// conditions if somebody accesses the URIs before these plugins are
	// registered
	domain.RegisterPlugin(func() domain.Plugin { return bmcSvc })
	domain.RegisterPlugin(func() domain.Plugin { return prot })
	domain.RegisterPlugin(func() domain.Plugin { return chas })
	domain.RegisterPlugin(func() domain.Plugin { return system })
	domain.RegisterPlugin(func() domain.Plugin { return therm })
	domain.RegisterPlugin(func() domain.Plugin { return temps })
	domain.RegisterPlugin(func() domain.Plugin { return fans })

	// and now add everything to the URI tree
	time.Sleep(250 * time.Millisecond) // still a small race in events, so sleep needed for now
	bmcSvc.AddResource(ctx, ch, eb, ew)
	time.Sleep(250 * time.Millisecond) // still a small race in events, so sleep needed for now
	prot.AddResource(ctx, ch)
	chas.AddResource(ctx, ch)
	system.AddResource(ctx, ch, eb, ew)
	therm.AddResource(ctx, ch, eb, ew)
	temps.AddResource(ctx, ch, eb, ew)
	fans.AddResource(ctx, ch, eb, ew)

	bmcSvc.ApplyOption(plugins.UpdateProperty("manager.reset", func(event eh.Event, res *domain.HTTPCmdProcessedData) {
		fmt.Printf("Hello WORLD!\n\tGOT RESET EVENT\n")
		res.Results = map[string]interface{}{"RESET": "FAKE SIMULATED BMC RESET"}
	}))

	system.ApplyOption(plugins.UpdateProperty("computersystem.reset", func(event eh.Event, res *domain.HTTPCmdProcessedData) {
		fmt.Printf("Hello WORLD!\n\tGOT RESET EVENT\n")
		res.Results = map[string]interface{}{"RESET": "FAKE SIMULATED COMPUTER RESET"}
	}))
}
