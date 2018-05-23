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
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/cmc.integrated"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/iom.slot"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/system.modular"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/system.chassis"
	"github.com/superchalupa/go-redfish/src/dell-resources/managers/cmc.integrated"
)

type ocp struct {
	rootSvc             *root.Service
	sessionSvc          *session.Service
	configChangeHandler func()
	logger              log.Logger
}

func (o *ocp) GetSessionSvc() *session.Service { return o.sessionSvc }
func (o *ocp) ConfigChangeHandler()            { o.configChangeHandler() }

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.Mutex, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) *ocp {
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

	var managers []*model.Model
	for _, mgrName := range []string{
		"CMC.Integrated.1",
		"CMC.Integrated.2",
	} {
		//************************
		// Create MANAGER objects for CMC.Integrated
		//************************
		mgrLogger := logger.New("module", "Managers/"+mgrName, "module", "Managers/CMC.Integrated")
		cmc_integrated_svc, _ := ec_manager.New(
			ec_manager.WithUniqueName(mgrName),
            model.UpdateProperty("name", ""),
            model.UpdateProperty("description", ""),
            model.UpdateProperty("model", ""),
            model.UpdateProperty("timezone", ""),
            model.UpdateProperty("firmware_version", ""),
            model.UpdateProperty("health_state", ""),
            model.UpdateProperty("redundancy_health_state", ""),
            model.UpdateProperty("redundancy_mode", ""),
            model.UpdateProperty("redundancy_min", ""),
            model.UpdateProperty("redundancy_max", ""),
		)
		managers = append(managers, cmc_integrated_svc)
		domain.RegisterPlugin(func() domain.Plugin { return cmc_integrated_svc })
		ec_manager.AddView(ctx, mgrLogger, cmc_integrated_svc, ch, eb, ew)
		updateFn, _ := generic_dell_resource.AddController(ctx, mgrLogger, cmc_integrated_svc, "Managers/"+mgrName, ch, eb, ew)
		updateFns = append(updateFns, updateFn)

		bmcAttrSvc, _ := attr_res.New(
			attr_res.BaseResource(cmc_integrated_svc),
			attr_res.WithURI("/redfish/v1/Managers/"+mgrName+"/Attributes"),
			attr_res.WithUniqueName(mgrName+".Attributes"),
		)
		domain.RegisterPlugin(func() domain.Plugin { return bmcAttrSvc })
		bmcAttrSvc.AddView(ctx, ch, eb, ew)

		bmcAttrProp, _ := attr_prop.New(
			attr_prop.BaseResource(bmcAttrSvc),
			attr_prop.WithFQDD(mgrName),
		)
		domain.RegisterPlugin(func() domain.Plugin { return bmcAttrProp })
		bmcAttrProp.AddView(ctx, ch, eb, ew)
		bmcAttrProp.AddController(ctx, ch, eb, ew)

		//************************
		// Create CHASSIS objects for CMC.Integrated
		//************************
		chasLogger := logger.New("module", "Chassis/"+mgrName, "module", "Chassis/CMC.Integrated")
		mgr_svc, _ := generic_chassis.New(
			ec_manager.WithUniqueName(mgrName),
			model.UpdateProperty("asset_tag", ""),
			model.UpdateProperty("serial", ""),
			model.UpdateProperty("part_number", ""),
			model.UpdateProperty("chassis_type", ""),
			model.UpdateProperty("model", ""),
			model.UpdateProperty("manufacturer", ""),
			model.UpdateProperty("name", ""),
		)
		domain.RegisterPlugin(func() domain.Plugin { return mgr_svc })
		cmc_chassis.AddView(ctx, chasLogger, mgr_svc, ch, eb, ew)
		// NOTE: looks like we can use the same mapping to model as manager object
		chasUpdateFn, _ := generic_dell_resource.AddController(ctx, chasLogger, mgr_svc, "Managers/"+mgrName, ch, eb, ew)
		updateFns = append(updateFns, chasUpdateFn)

		mgrAttrSvc, _ := attr_res.New(
			attr_res.BaseResource(mgr_svc),
			attr_res.WithURI("/redfish/v1/Chassis/"+mgrName+"/Attributes"),
			attr_res.WithUniqueName(mgrName+".Attributes"),
		)
		domain.RegisterPlugin(func() domain.Plugin { return mgrAttrSvc })
		mgrAttrSvc.AddView(ctx, ch, eb, ew)

		// TODO: would be nice if we could re-use the underlying model between the manager and chassis object
		//       should be do-able if we modify BaseResource() to be AttachToResource(), and make the underlying data
		//       an array
		mgrAttrProp, _ := attr_prop.New(
			attr_prop.BaseResource(mgrAttrSvc),
			attr_prop.WithFQDD(mgrName),
		)
		domain.RegisterPlugin(func() domain.Plugin { return mgrAttrProp })
		mgrAttrProp.AddView(ctx, ch, eb, ew)
		mgrAttrProp.AddController(ctx, ch, eb, ew)
	}


    // ********************
    // System.Chassis.1
    // ********************
    chasName := "System.Chassis.1"
	chasLogger := logger.New("module", "Chassis/"+chasName, "module", "Chassis/System.Chassis")
    chasSvc, _ := generic_chassis.New(
        ec_manager.WithUniqueName(chasName),
		generic_chassis.AddManagedBy(managers[0]),
        model.UpdateProperty("asset_tag", ""),
        model.UpdateProperty("serial", ""),
        model.UpdateProperty("part_number", ""),
        model.UpdateProperty("chassis_type", ""),
        model.UpdateProperty("model", ""),
        model.UpdateProperty("manufacturer", ""),
        model.UpdateProperty("name", ""),
        model.UpdateProperty("description", ""),
        model.UpdateProperty("power_state", ""),
    )
    domain.RegisterPlugin(func() domain.Plugin { return chasSvc })
    system_chassis.AddView(ctx, chasLogger, chasSvc, ch, eb, ew)
    chasUpdateFn, _ := generic_dell_resource.AddController(ctx, chasLogger, chasSvc, "Chassis/"+chasName, ch, eb, ew)
    updateFns = append(updateFns, chasUpdateFn)



	for _, iomName := range []string{
		"IOM.Slot.A1", "IOM.Slot.A1a", "IOM.Slot.A1b",
		"IOM.Slot.A2", "IOM.Slot.A2a", "IOM.Slot.A2b",
		"IOM.Slot.B1", "IOM.Slot.B1a", "IOM.Slot.B1b",
		"IOM.Slot.B2", "IOM.Slot.B2a", "IOM.Slot.B2b",
		"IOM.Slot.C1",
		"IOM.Slot.C2",
	} {
		iomLogger := logger.New("module", "Chassis/"+iomName, "module", "Chassis/IOM.Slot")
		iom, _ := generic_chassis.New(
			generic_chassis.WithUniqueName(iomName),
			generic_chassis.AddManagedBy(managers[0]),

			// TODO: maybe the mapper could add these automatically?
			model.UpdateProperty("service_tag", ""),
			model.UpdateProperty("asset_tag", ""),
			model.UpdateProperty("description", ""),
			model.UpdateProperty("power_state", ""),
			model.UpdateProperty("serial", ""),
			model.UpdateProperty("part_number", ""),
			model.UpdateProperty("chassis_type", ""),
			model.UpdateProperty("model", ""),
			model.UpdateProperty("name", ""),
			model.UpdateProperty("manufacturer", ""),
		)
		domain.RegisterPlugin(func() domain.Plugin { return iom })
		iom_chassis.AddView(ctx, iomLogger, iom, ch, eb, ew)

		updateFn, _ := generic_dell_resource.AddController(ctx,
			logger.New("module", "Chassis/"+iomName, "module", "Chassis/IOM.Slot"),
			iom, "Managers/"+iomName, ch, eb, ew)
		updateFns = append(updateFns, updateFn)

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
		"System.Modular.1", "System.Modular.1a", "System.Modular.1b",
		"System.Modular.2", "System.Modular.2a", "System.Modular.2b",
		"System.Modular.3", "System.Modular.3a", "System.Modular.3b",
		"System.Modular.4", "System.Modular.4a", "System.Modular.4b",
		"System.Modular.5", "System.Modular.5a", "System.Modular.5b",
		"System.Modular.6", "System.Modular.6a", "System.Modular.6b",
		"System.Modular.7", "System.Modular.7a", "System.Modular.7b",
		"System.Modular.8", "System.Modular.8a", "System.Modular.8b",
	} {
		sledLogger := logger.New("module", "Chassis/System.Modular", "module", "Chassis/"+sledName)
		sled, _ := generic_chassis.New(
			generic_chassis.WithUniqueName(sledName),
			generic_chassis.AddManagedBy(managers[0]),
			model.UpdateProperty("service_tag", ""),
			model.UpdateProperty("power_state", ""),
			model.UpdateProperty("chassis_type", ""),
			model.UpdateProperty("model", ""),
			model.UpdateProperty("manufacturer", ""),
			model.UpdateProperty("serial", ""),
		)
		domain.RegisterPlugin(func() domain.Plugin { return sled })
		sled_chassis.AddView(sled, ctx, ch, eb, ew)
		updateFn, _ := generic_dell_resource.AddController(ctx, sledLogger, sled, "Chassis/"+sledName, ch, eb, ew)
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
