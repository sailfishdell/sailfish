// Build tags: only build this for the simulation build. Be sure to note the required blank line after.
// +build simulation

package obmc

import (
	"context"
	"fmt"
	"sync"

	"github.com/spf13/viper"
	"io/ioutil"
	// "github.com/go-yaml/yaml"
	yaml "gopkg.in/yaml.v2"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	"github.com/superchalupa/go-redfish/src/log"
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

type ocp struct {
	rootSvc             *root.Service
	sessionSvc          *session.Service
	basicAuthSvc        *basicauth.Service
	configChangeHandler func()
	logger              log.Logger
}

func (o *ocp) GetSessionSvc() *session.Service     { return o.sessionSvc }
func (o *ocp) GetBasicAuthSvc() *basicauth.Service { return o.basicAuthSvc }
func (o *ocp) ConfigChangeHandler()                { o.configChangeHandler() }

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.Mutex, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) *ocp {
	// initial implementation is one BMC, one Chassis, and one System.
	// Yes, this function is somewhat long, however there really isn't any logic here. If we start getting logic, this needs to be split.

	logger = logger.New("module", "ocp_SIMULATION")
	self := &ocp{
		logger: logger,
	}

	self.rootSvc, _ = root.New()

	self.sessionSvc, _ = session.New(
		session.Root(self.rootSvc),
	)

	self.basicAuthSvc, _ = basicauth.New()

	bmcSvc, _ := bmc.New(
		bmc.WithUniqueName("OBMC"),
	)

	protocolSvc, _ := protocol.New(
		protocol.WithBMC(bmcSvc),
	)

	chas, _ := chassis.New(
		chassis.AddManagedBy(bmcSvc),
		chassis.AddManagerInChassis(bmcSvc),
		chassis.WithUniqueName("Chassis.Embedded.1"),
	)

	bmcSvc.InChassis(chas)
	bmcSvc.AddManagerForChassis(chas)

	system, _ := system.New(
		system.WithUniqueName("System.Embedded.1"),
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

	fanObj, _ := fans.New(
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

	// VIPER Config:
	// pull the config from the YAML file to populate some static config options
	self.configChangeHandler = func() {
		logger.Info("Re-applying configuration from config file.")

		self.sessionSvc.ApplyOption(plugins.UpdateProperty("session_timeout", cfgMgr.GetInt("session.timeout")))

		for _, k := range []string{"name", "description", "model", "timezone", "version"} {
			bmcSvc.ApplyOption(plugins.UpdateProperty(k, cfgMgr.Get("managers."+bmcSvc.GetUniqueName()+"."+k)))
		}

		for _, m := range cfgMgr.Get("managers." + bmcSvc.GetUniqueName() + ".proto").([]interface{}) {
			options := map[string]interface{}{}

			prot, ok := m.(map[interface{}]interface{})
			if !ok {
				logger.Debug("type assert failure - prot", "prot", prot, "ok", ok, "type", fmt.Sprintf("%T", prot), "raw", m)
				continue
			}

			name, ok := prot["name"].(string)
			if !ok {
				logger.Debug("type assert failure - name", "name", name, "ok", ok)
				continue
			}

			enabled, ok := prot["enabled"].(bool)
			if !ok {
				logger.Debug("type assert failure - enabled", "enabled", enabled, "ok", ok)
				continue
			}

			port, ok := prot["port"].(int)
			if !ok {
				logger.Debug("type assert failure - port", "port", port, "ok", ok)
				continue
			}

			opts, ok := prot["options"].([]interface{})
			if !ok {
				opts = []interface{}{}
			}

			for _, m := range opts {
				o := m.(map[interface{}]interface{})
				name, ok := o["name"].(string)
				if !ok {
					continue
				}
				value, ok := o["value"]
				if !ok {
					continue
				}

				options[name] = value
			}

			// TODO: better error checks on type assertions...
			logger.Debug("Add protocol", "protocol", name, "enabled", enabled, "port", port, "options", options)
			protocolSvc.ApplyOption(protocol.WithProtocol(name, enabled, port, options))
		}

		for _, k := range []string{
			"name", "chassis_type", "model",
			"serial_number", "sku", "part_number",
			"asset_tag", "chassis_type", "manufacturer"} {
			chas.ApplyOption(plugins.UpdateProperty(k, cfgMgr.Get("chassis."+chas.GetUniqueName()+"."+k)))
		}
		for _, k := range []string{
			"name", "system_type", "asset_tag", "manufacturer",
			"model", "serial_number", "sku", "The SKU", "part_number",
			"description", "power_state", "bios_version", "led", "system_hostname",
		} {
			system.ApplyOption(plugins.UpdateProperty(k, cfgMgr.Get("systems."+system.GetUniqueName()+"."+k)))
		}
	}
	self.ConfigChangeHandler()

	cfgMgr.SetDefault("main.dumpConfigChanges.filename", "redfish-changed.yaml")
	cfgMgr.SetDefault("main.dumpConfigChanges.enabled", "true")
	dumpViperConfig := func() {
		viperMu.Lock()
		defer viperMu.Unlock()

		dumpFileName := cfgMgr.GetString("main.dumpConfigChanges.filename")
		enabled := cfgMgr.GetBool("main.dumpConfigChanges.enabled")
		if !enabled {
			return
		}

		// TODO: change this to a streaming write (reduce mem usage)
		var config map[string]interface{}
		cfgMgr.Unmarshal(&config)
		output, _ := yaml.Marshal(config)
		_ = ioutil.WriteFile(dumpFileName, output, 0644)
	}

	self.sessionSvc.AddPropertyObserver("session_timeout", func(newval interface{}) {
		viperMu.Lock()
		cfgMgr.Set("session.timeout", newval.(int))
		viperMu.Unlock()
		dumpViperConfig()
	})

	// register all of the plugins (do this first so we dont get any race
	// conditions if somebody accesses the URIs before these plugins are
	// registered
	domain.RegisterPlugin(func() domain.Plugin { return self.rootSvc })
	domain.RegisterPlugin(func() domain.Plugin { return self.sessionSvc })
	domain.RegisterPlugin(func() domain.Plugin { return self.basicAuthSvc })
	domain.RegisterPlugin(func() domain.Plugin { return bmcSvc })
	domain.RegisterPlugin(func() domain.Plugin { return protocolSvc })
	domain.RegisterPlugin(func() domain.Plugin { return chas })
	domain.RegisterPlugin(func() domain.Plugin { return system })
	domain.RegisterPlugin(func() domain.Plugin { return therm })
	domain.RegisterPlugin(func() domain.Plugin { return temps })
	domain.RegisterPlugin(func() domain.Plugin { return fanObj })

	// and now add everything to the URI tree
	self.rootSvc.AddResource(ctx, ch, eb, ew)
	self.sessionSvc.AddResource(ctx, ch, eb, ew)
	self.basicAuthSvc.AddResource(ctx, ch, eb, ew)
	bmcSvc.AddResource(ctx, ch, eb, ew)
	protocolSvc.AddResource(ctx, ch)
	chas.AddResource(ctx, ch)
	system.AddResource(ctx, ch, eb, ew)
	therm.AddResource(ctx, ch, eb, ew)
	temps.AddResource(ctx, ch, eb, ew)
	fanObj.AddResource(ctx, ch, eb, ew)

	bmcSvc.ApplyOption(plugins.UpdateProperty("manager.reset", func(event eh.Event, res *domain.HTTPCmdProcessedData) {
		self.logger.Crit("Hello WORLD!\n\tGOT RESET EVENT\n")
		res.Results = map[string]interface{}{"RESET": "FAKE SIMULATED BMC RESET"}
	}))

	system.ApplyOption(plugins.UpdateProperty("computersystem.reset", func(event eh.Event, res *domain.HTTPCmdProcessedData) {
		self.logger.Crit("Hello WORLD!\n\tGOT RESET EVENT\n")
		res.Results = map[string]interface{}{"RESET": "FAKE SIMULATED COMPUTER RESET"}
	}))

	return self
}
