// Build tags: only build this for the simulation build. Be sure to note the required blank line after.
// +build openbmc

package obmc

import (
	"context"
	"fmt"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"io/ioutil"
	// "github.com/go-yaml/yaml"
	yaml "gopkg.in/yaml.v2"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	plugins "github.com/superchalupa/go-redfish/src/ocp"
	"github.com/superchalupa/go-redfish/src/ocp/basicauth"
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

func InitOCP(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) (*session.Service, *basicauth.Service) {
	// initial implementation is one BMC, one Chassis, and one System. If we
	// expand beyond that, we need to adjust stuff here.

	rootSvc, _ := root.New(
		plugins.UpdateProperty("test", "test property"),
	)

	sessionSvc, _ := session.New(
		session.Root(rootSvc),
	)
	basicAuthSvc, _ := basicauth.New()

	bmcSvc, _ := bmc.New(
		bmc.WithUniqueName("OBMC"),
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
	)

	bmcSvc.InChassis(chas)
	bmcSvc.AddManagerForChassis(chas)

	system, _ := system.New(
		system.WithUniqueName("1"),
		system.ManagedBy(bmcSvc),
		system.InChassis(chas),
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

	// VIPER Config:
	// pull the config from the YAML file to populate some static config options
	pullViperConfig := func() {
		sessionSvc.ApplyOption(plugins.UpdateProperty("session_timeout", viper.GetInt("session.timeout")))
		for _, k := range []string{"name", "description", "model", "timezone", "version"} {
			bmcSvc.ApplyOption(plugins.UpdateProperty(k, viper.Get("managers.OBMC."+k)))
		}
		for _, k := range []string{
			"name", "chassis_type", "model",
			"serial_number", "sku", "part_number",
			"asset_tag", "chassis_type", "manufacturer"} {
			chas.ApplyOption(plugins.UpdateProperty(k, viper.Get("chassis.1."+k)))
		}
		for _, k := range []string{
			"name", "system_type", "asset_tag", "manufacturer",
			"model", "serial_number", "sku", "The SKU", "part_number",
			"description", "power_state", "bios_version", "led", "system_hostname",
		} {
			system.ApplyOption(plugins.UpdateProperty(k, viper.Get("systems.1."+k)))
		}
	}
	pullViperConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
		pullViperConfig()
	})
	viper.WatchConfig()

	dumpMu = sync.Mutex
	dumpViperConfig := func() {
		dumpMu.Lock()
		defer dumpMu.Unlock()

		var config map[string]interface{}
		viper.Unmarshal(&config)

		output, _ := yaml.Marshal(config)
		_ = ioutil.WriteFile("output.yaml", output, 0644)
	}

	sessionSvc.AddPropertyObserver("session_timeout", func(newval interface{}) {
		fmt.Printf("\nSESSION TIMEOUT CHANGED\n\n")
		viper.Set("session.timeout", newval.(int))
		dumpViperConfig()
	})

	// register all of the plugins (do this first so we dont get any race
	// conditions if somebody accesses the URIs before these plugins are
	// registered
	domain.RegisterPlugin(func() domain.Plugin { return rootSvc })
	domain.RegisterPlugin(func() domain.Plugin { return sessionSvc })
	domain.RegisterPlugin(func() domain.Plugin { return basicAuthSvc })
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
	basicAuthSvc.AddResource(ctx, ch, eb, ew)
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

	return sessionSvc, basicAuthSvc
}
