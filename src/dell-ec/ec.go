// Build tags: only build this for the simulation build. Be sure to note the required blank line after.
// +build ec

package obmc

import (
	"context"
	"sync"
	"time"

	"github.com/spf13/viper"
	"io/ioutil"
	// "github.com/go-yaml/yaml"
	yaml "gopkg.in/yaml.v2"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	"github.com/superchalupa/go-redfish/src/ocp/root"
	"github.com/superchalupa/go-redfish/src/ocp/session"

	attr_prop "github.com/superchalupa/go-redfish/src/dell-resources/attribute-property"
	attr_res "github.com/superchalupa/go-redfish/src/dell-resources/attribute-resource"

	"github.com/superchalupa/go-redfish/src/dell-resources"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/iom.slot"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/system.modular"
	"github.com/superchalupa/go-redfish/src/dell-resources/managers/cmc.integrated"
)

type ocp struct {
	rootSvc             *root.Service
	sessionSvc          *session.Service
	configChangeHandler func()
	logger              log.Logger
}

func (o *ocp) GetSessionSvc() *session.Service     { return o.sessionSvc }
func (o *ocp) ConfigChangeHandler()                { o.configChangeHandler() }

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.Mutex, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) *ocp {
	// initial implementation is one BMC, one Chassis, and one System.
	// Yes, this function is somewhat long, however there really isn't any logic here. If we start getting logic, this needs to be split.

	logger = logger.New("module", "ec")
	self := &ocp{
		logger: logger,
	}

	updateFns := []func(*viper.Viper){}

	self.rootSvc, _ = root.New()
	domain.RegisterPlugin(func() domain.Plugin { return self.rootSvc })
	root.AddView(ctx, self.rootSvc, ch, eb, ew)
	time.Sleep(1)

	self.sessionSvc, _ = session.New(
		session.Root(self.rootSvc),
	)
	domain.RegisterPlugin(func() domain.Plugin { return self.sessionSvc })
	self.sessionSvc.AddResource(ctx, ch, eb, ew)

	cmc_integrated_1_svc, _ := ec_manager.New(
		ec_manager.WithUniqueName("CMC.Integrated.1"),
	)
	domain.RegisterPlugin(func() domain.Plugin { return cmc_integrated_1_svc })
	cmc_integrated_1_svc.AddView(ctx, ch, eb, ew)
	update1Fn, _ := generic_dell_resource.AddController(ctx, logger.New("module", "Managers/CMC.Integrated.1"), cmc_integrated_1_svc.Service, "Managers/CMC.Integrated.1", ch, eb, ew)
	updateFns = append(updateFns, update1Fn)

	bmcAttrSvc, _ := attr_res.New(
		attr_res.BaseResource(cmc_integrated_1_svc),
		attr_res.WithURI("/redfish/v1/Managers/CMC.Integrated.1/Attributes"),
		attr_res.WithUniqueName("CMC.Integrated.1.Attributes"),
	)
	domain.RegisterPlugin(func() domain.Plugin { return bmcAttrSvc })
	bmcAttrSvc.AddView(ctx, ch, eb, ew)

	bmcAttrProp, _ := attr_prop.New(
		attr_prop.BaseResource(bmcAttrSvc),
		attr_prop.WithFQDD("CMC.Integrated.1"),
	)
	domain.RegisterPlugin(func() domain.Plugin { return bmcAttrProp })
	bmcAttrProp.AddView(ctx, ch, eb, ew)
	bmcAttrProp.AddController(ctx, ch, eb, ew)

	cmc_integrated_2_svc, _ := ec_manager.New(
		ec_manager.WithUniqueName("CMC.Integrated.2"),
	)
	domain.RegisterPlugin(func() domain.Plugin { return cmc_integrated_2_svc })
	cmc_integrated_2_svc.AddView(ctx, ch, eb, ew)
	update2Fn, _ := generic_dell_resource.AddController(ctx, logger.New("module", "Managers/CMC.Integrated.2"), cmc_integrated_2_svc.Service, "Managers/CMC.Integrated.2", ch, eb, ew)
	updateFns = append(updateFns, update2Fn)

	bmcAttr2Svc, _ := attr_res.New(
		attr_res.BaseResource(cmc_integrated_2_svc),
		attr_res.WithURI("/redfish/v1/Managers/CMC.Integrated.2/Attributes"),
		attr_res.WithUniqueName("CMC.Integrated.2.Attributes"),
	)
	domain.RegisterPlugin(func() domain.Plugin { return bmcAttr2Svc })
	bmcAttr2Svc.AddView(ctx, ch, eb, ew)

	bmcAttr2Prop, _ := attr_prop.New(
		attr_prop.BaseResource(bmcAttr2Svc),
		attr_prop.WithFQDD("CMC.Integrated.2"),
	)
	domain.RegisterPlugin(func() domain.Plugin { return bmcAttr2Prop })
	bmcAttr2Prop.AddView(ctx, ch, eb, ew)
	bmcAttr2Prop.AddController(ctx, ch, eb, ew)

	for _, iomName := range []string{
		"IOM.Slot.A1",
		"IOM.Slot.A1a",
		"IOM.Slot.A1b",
		"IOM.Slot.A2",
		"IOM.Slot.A2a",
		"IOM.Slot.A2b",
		"IOM.Slot.B1",
		"IOM.Slot.B1a",
		"IOM.Slot.B1b",
		"IOM.Slot.B2",
		"IOM.Slot.B2a",
		"IOM.Slot.B2b",
		"IOM.Slot.C1",
		"IOM.Slot.C2",
	} {
		iom, _ := generic_chassis.New(
			generic_chassis.WithUniqueName(iomName),
			generic_chassis.AddManagedBy(cmc_integrated_1_svc),
		)
		domain.RegisterPlugin(func() domain.Plugin { return iom })
		iom_chassis.AddView(ctx, iom, ch, eb, ew)

		iomAttrSvc, _ := attr_res.New(
			attr_res.BaseResource(iom),
			attr_res.WithURI("/redfish/v1/Chassis/"+iomName+"/Attributes"),
			attr_res.WithUniqueName(iomName+".Attributes"),
		)
		domain.RegisterPlugin(func() domain.Plugin { return iomAttrSvc })
		iomAttrSvc.AddView(ctx, ch, eb, ew)
		iomAttrSvc.AddController(ctx, ch, eb, ew)

		iomAttrProp, _ := attr_prop.New(
			attr_prop.BaseResource(iomAttrSvc),
			attr_prop.WithFQDD(iomName),
		)
		domain.RegisterPlugin(func() domain.Plugin { return iomAttrProp })
		iomAttrProp.AddView(ctx, ch, eb, ew)
		iomAttrProp.AddController(ctx, ch, eb, ew)
	}

	for _, sledName := range []string{
		"System.Modular.1",
		"System.Modular.1a",
		"System.Modular.1b",
		"System.Modular.2",
		"System.Modular.2a",
		"System.Modular.2b",
		"System.Modular.3",
		"System.Modular.3a",
		"System.Modular.3b",
		"System.Modular.4",
		"System.Modular.4a",
		"System.Modular.4b",
		"System.Modular.5",
		"System.Modular.5a",
		"System.Modular.5b",
		"System.Modular.6",
		"System.Modular.6a",
		"System.Modular.6b",
		"System.Modular.7",
		"System.Modular.7a",
		"System.Modular.7b",
		"System.Modular.8",
		"System.Modular.8a",
		"System.Modular.8b",
	} {
		sled, _ := generic_chassis.New(
			generic_chassis.WithUniqueName(sledName),
			generic_chassis.AddManagedBy(cmc_integrated_1_svc),
			model.UpdateProperty("service_tag", ""),
			model.UpdateProperty("power_state", ""),
		)
		domain.RegisterPlugin(func() domain.Plugin { return sled })
		sled_chassis.AddView(sled, ctx, ch, eb, ew)
		updateFn, _ := generic_dell_resource.AddController(ctx, logger.New("module", "Chassis/System.Modular", "module", "Chassis/"+sledName), sled, "Chassis/"+sledName, ch, eb, ew)
		updateFns = append(updateFns, updateFn)

		sledAttrSvc, _ := attr_res.New(
			attr_res.BaseResource(sled),
			attr_res.WithURI("/redfish/v1/Chassis/"+sledName+"/Attributes"),
			attr_res.WithUniqueName(sledName+".Attributes"),
		)
		domain.RegisterPlugin(func() domain.Plugin { return sledAttrSvc })
		sledAttrSvc.AddView(ctx, ch, eb, ew)
		sledAttrSvc.AddController(ctx, ch, eb, ew)

		sledAttrProp, _ := attr_prop.New(
			attr_prop.BaseResource(sledAttrSvc),
			attr_prop.WithFQDD(sledName),
		)
		domain.RegisterPlugin(func() domain.Plugin { return sledAttrProp })
		sledAttrProp.AddView(ctx, ch, eb, ew)
		sledAttrProp.AddController(ctx, ch, eb, ew)
	}

	// VIPER Config:
	// pull the config from the YAML file to populate some static config options
	self.configChangeHandler = func() {
		logger.Info("Re-applying configuration from config file.")
		self.sessionSvc.ApplyOption(model.UpdateProperty("session_timeout", cfgMgr.GetInt("session.timeout")))

		for _, fn := range updateFns {
			fn(cfgMgr)
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

	return self
}
