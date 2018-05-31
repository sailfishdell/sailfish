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

	"github.com/superchalupa/go-redfish/src/dell-resources/ar_mapper"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/cmc.integrated"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/iom.slot"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/system.chassis"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/system.chassis/power"
	//	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/system.chassis/power/powersupply"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/system.chassis/thermal"
	//	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/system.chassis/thermal/fans"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/system.modular"
	"github.com/superchalupa/go-redfish/src/dell-resources/managers/cmc.integrated"
	"github.com/superchalupa/go-redfish/src/dell-resources/test"
)

type ocp struct {
	rootModel           *root.Service
	sessionModel        *session.Service
	configChangeHandler func()
	logger              log.Logger
}

func (o *ocp) GetSessionModel() *session.Service { return o.sessionModel }
func (o *ocp) ConfigChangeHandler()              { o.configChangeHandler() }

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.Mutex, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) *ocp {
	logger = logger.New("module", "ec")
	self := &ocp{
		logger: logger,
	}

	updateFns := []func(context.Context, *viper.Viper){}

	//
	// Create the (empty) model behind the /redfish/v1 service root. Nothing interesting here
	//
	self.rootModel, _ = root.New()
	domain.RegisterPlugin(func() domain.Plugin { return self.rootModel })
	root.AddView(ctx, self.rootModel, ch, eb, ew)

	//
	// temporary workaround: we create /redfish/v1/{Chassis,Managers,Systems,Accounts}, etc in the background and that can race, so stop here for a sec.
	//
	time.Sleep(1)

	//
	// Create the /redfish/v1/Sessions model and handler
	//
	self.sessionModel, _ = session.New(
		session.Root(self.rootModel),
	)
	domain.RegisterPlugin(func() domain.Plugin { return self.sessionModel })
	self.sessionModel.AddResource(ctx, ch, eb, ew)

	// construction order:
	//   1) model
	//   2) controller(s) - pass model by args
	//   3) views - pass models and controllers by args
	//
	testLogger := logger.New("module", "TEST")
	testModel := model.NewModel(
		model.UpdateProperty("unique_name", "un"),
		model.UpdateProperty("name", "n"),
		model.UpdateProperty("description", "d"),
		model.UpdateProperty("model", "m"),
	)
	testController, _ := ar_mapper.NewARMappingController(ctx, testLogger, testModel, "test/testview", ch, eb, ew)
	updateFns = append(updateFns, testController.ConfigChangedFn)
	test.NewView(ctx, testModel, testController, ch)

	//
	// Loop to create similarly named manager objects and the things attached there.
	//
	var managers []*model.Model
	for _, mgrName := range []string{
		"CMC.Integrated.1",
		"CMC.Integrated.2",
	} {
		//*********************************************************************
		// Create MANAGER objects for CMC.Integrated.N
		//*********************************************************************
		mgrLogger := logger.New("module", "Managers/"+mgrName, "module", "Managers/CMC.Integrated")
		cmcIntegratedModel, _ := ec_manager.New(
			ec_manager.WithUniqueName(mgrName),
			model.UpdateProperty("name", nil),
			model.UpdateProperty("description", nil),
			model.UpdateProperty("model", nil),
			model.UpdateProperty("timezone", nil),
			model.UpdateProperty("firmware_version", nil),
			model.UpdateProperty("health_state", nil),
			model.UpdateProperty("redundancy_health_state", nil),
			model.UpdateProperty("redundancy_mode", nil),
			model.UpdateProperty("redundancy_min", nil),
			model.UpdateProperty("redundancy_max", nil),
			model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),
		)
		// save managers because we use these later
		managers = append(managers, cmcIntegratedModel)

		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		mgrController, _ := ar_mapper.NewARMappingController(ctx, mgrLogger, cmcIntegratedModel, "Managers/"+mgrName, ch, eb, ew)

		// This controller will populate 'attributes' property with AR entries matching this FQDD ('mgrName')
		mgrArdump, _ := attr_prop.NewController(ctx, cmcIntegratedModel, []string{mgrName}, ch, eb, ew)

		// let the controller re-read its mappings when config file changes... nifty
		updateFns = append(updateFns, mgrController.ConfigChangedFn)

		// add the actual view
		ec_manager.AddView(ctx, mgrLogger, cmcIntegratedModel, mgrController, ch, eb, ew)

		// Create the .../Attributes URI. Attributes are stored in the attributes property
		v := attr_prop.NewView(ctx, cmcIntegratedModel, mgrArdump)
		mgrUUID := attr_res.AddView(ctx, "/redfish/v1/Managers/"+mgrName+"/Attributes", mgrName+".Attributes", ch, eb, ew)
		attr_prop.EnhanceExistingUUID(ctx, v, ch, mgrUUID)

		//*********************************************************************
		// Create CHASSIS objects for CMC.Integrated.N
		//*********************************************************************
		chasLogger := logger.New("module", "Chassis/"+mgrName, "module", "Chassis/CMC.Integrated")
		chasModel, _ := generic_chassis.New(
			ec_manager.WithUniqueName(mgrName),
			model.UpdateProperty("asset_tag", ""),
			model.UpdateProperty("serial", ""),
			model.UpdateProperty("part_number", ""),
			model.UpdateProperty("chassis_type", ""),
			model.UpdateProperty("model", ""),
			model.UpdateProperty("manufacturer", ""),
			model.UpdateProperty("name", ""),
			model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),
		)
		// This controller will populate 'attributes' property with AR entries matching this FQDD ('mgrName')
		chasArdump, _ := attr_prop.NewController(ctx, chasModel, []string{mgrName}, ch, eb, ew)

		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		chasController, _ := ar_mapper.NewARMappingController(ctx, chasLogger, chasModel, "Managers/"+mgrName, ch, eb, ew)
		updateFns = append(updateFns, chasController.ConfigChangedFn)

		cmc_chassis.AddView(ctx, chasLogger, chasModel, chasController, ch, eb, ew)

		// Create the .../Attributes URI. Attributes are stored in the attributes property of the chasModel
		v2 := attr_prop.NewView(ctx, chasModel, chasArdump)
		chasUUID := attr_res.AddView(ctx, "/redfish/v1/Chassis/"+mgrName+"/Attributes", mgrName+".Attributes", ch, eb, ew)
		attr_prop.EnhanceExistingUUID(ctx, v2, ch, chasUUID)
	}

    chasName := "System.Chassis.1"
    chasLogger := logger.New("module", "Chassis/"+chasName, "module", "Chassis/System.Chassis")
    {
        // ************************************************************************
        // CHASSIS System.Chassis.1
        // ************************************************************************
        chasModel, _ := generic_chassis.New(
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
			model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),
        )
        // This controller will populate 'attributes' property with AR entries matching this FQDD ('chasName')
        chasArdump, _ := attr_prop.NewController(ctx, chasModel, []string{chasName}, ch, eb, ew)

        // the controller is what updates the model when ar entries change,
        // also handles patch from redfish
        chasController, _ := ar_mapper.NewARMappingController(ctx, chasLogger, chasModel, "Chassis/"+chasName, ch, eb, ew)
        updateFns = append(updateFns, chasController.ConfigChangedFn)

        system_chassis.AddView(ctx, chasLogger, chasModel, chasController, ch, eb, ew)

        // Create the .../Attributes URI. Attributes are stored in the attributes property of the chasModel
        v2 := attr_prop.NewView(ctx, chasModel, chasArdump)
        chasUUID := attr_res.AddView(ctx, "/redfish/v1/Chassis/"+chasName+"/Attributes", chasName+".Attributes", ch, eb, ew)
        attr_prop.EnhanceExistingUUID(ctx, v2, ch, chasUUID)
    }


	//*********************************************************************
	// Create Power objects for System.Chassis.1
	//*********************************************************************
	powerLogger := chasLogger.New("module", "power")

	powerModel := model.NewModel(
		ec_manager.WithUniqueName("Power"),
		model.UpdateProperty("power_supply_views", []interface{}{}),
		model.UUID(),
		model.PluginType(domain.PluginType("Power:"+chasName)),
		model.PropertyOnce("uri", "/redfish/v1/Chassis/"+chasName+"/Power"),
	)
	domain.RegisterPlugin(func() domain.Plugin { return powerModel })
	power.AddView(ctx, powerLogger, powerModel, ch, eb, ew)

	/*
		psus := []interface{}{}
		for _, psuName := range []string{
			"PSU.Slot.1", "PSU.Slot.2", "PSU.Slot.3",
			"PSU.Slot.4", "PSU.Slot.5", "PSU.Slot.6",
		} {
			// note we don't AddView for this one because we already have this in our view
			psuAttrProp, _ := attr_prop.New(
				attr_prop.WithFQDD(psuName),
			)
			domain.RegisterPlugin(func() domain.Plugin { return psuAttrProp })
			psuAttrProp.AddController(ctx, ch, eb, ew)

			psModel := model.NewModel(
				model.UUID(),
				model.PropertyOnce("uri", "/redfish/v1/Chassis/"+chasName+"/Power/PowerSupplies/"+psuName),
				model.PluginType(domain.PluginType("PowerSupply:"+psuName)),
				model.UpdateProperty("unique_id", psuName),
				model.UpdateProperty("name", psuName),
				model.UpdateProperty("capacity_watts", "INVALID"),
				model.UpdateProperty("firmware_version", "NOT INVENTORIED"),
				model.UpdateProperty("component_id", "INVALID"),
				model.UpdateProperty("line_input_voltage", ""),
				model.UpdateProperty("input_current", ""),
			)
			domain.RegisterPlugin(func() domain.Plugin { return psModel })
			psu := powersupply.AddView(ctx, powerLogger, psModel, psuAttrProp.GetModel(), ch, eb, ew)

			psuController, _ := ar_mapper.NewARMappingController(ctx,
				logger.New("module", "Chassis/"+chasName+"/Power/PowerSupplies/"+psuName, "module", "Chassis/"+chasName+"/Power/PowerSupplies/PSU.Slot"),
				psModel, "PSU/"+psuName, ch, eb, ew)
			updateFns = append(updateFns, psuController.ConfigChangedFn)

			p := &domain.RedfishResourceProperty{}
			p.Parse(psu)
			psus = append(psus, p)

		}
		p := &domain.RedfishResourceProperty{Value: psus}
		powerModel.ApplyOption(model.UpdateProperty("power_supply_views", p))
		powerLogger.Info("Updating view with psu list", "power_supply_views", p, "raw", psus)
	*/

	//*********************************************************************
	// Create Thermal objects for System.Chassis.1
	//*********************************************************************
	thermalLogger := chasLogger.New("module", "thermal")

	thermalModel := model.NewModel(
		ec_manager.WithUniqueName("Thermal"),
		model.UpdateProperty("fan_views", []interface{}{}),
		model.UpdateProperty("thermal_views", []interface{}{}),
		model.UUID(),
		model.PluginType(domain.PluginType("Thermal:"+chasName)),
		model.PropertyOnce("uri", "/redfish/v1/Chassis/"+chasName+"/Thermal"),
	)
	domain.RegisterPlugin(func() domain.Plugin { return thermalModel })
	thermal.AddView(ctx, thermalLogger, thermalModel, ch, eb, ew)

	/*
		fan_views := []interface{}{}
		for _, fanName := range []string{
			"Fan.Slot.1", "Fan.Slot.2", "Fan.Slot.3",
			"Fan.Slot.4", "Fan.Slot.5", "Fan.Slot.6",
			"Fan.Slot.7", "Fan.Slot.8", "Fan.Slot.9",
		} {

			// note we don't AddView for this one because we already have this in our view
			fanAttrProp, _ := attr_prop.New(
				attr_prop.WithFQDD(fanName),
			)
			domain.RegisterPlugin(func() domain.Plugin { return fanAttrProp })
			fanAttrProp.AddController(ctx, ch, eb, ew)

			psModel := model.NewModel(
				model.UUID(),
				model.PropertyOnce("uri", "/redfish/v1/Chassis/"+chasName+"/Sensors/Fans/"+fanName),
				model.PluginType(domain.PluginType("Fans:"+fanName)),
				model.UpdateProperty("unique_id", fanName),
				model.UpdateProperty("name", "UNSET"),
				model.UpdateProperty("firmware_version", "UNSET"),
				model.UpdateProperty("hardware_version", "UNSET"),
				model.UpdateProperty("reading", "UNSET"),
				model.UpdateProperty("reading_units", "UNSET"),
				model.UpdateProperty("oem_reading", "UNSET"),
				model.UpdateProperty("oem_reading_units", "UNSET"),
				model.UpdateProperty("graphics_uri", "UNSET"),
			)
			domain.RegisterPlugin(func() domain.Plugin { return psModel })
			fan := fans.AddView(ctx, thermalLogger, psModel, fanAttrProp.GetModel(), ch, eb, ew)

			fanController, _ := ar_mapper.NewARMappingController(ctx,
				logger.New("module", "Chassis/"+chasName+"/Sensors/Fans/"+fanName, "module", "Chassis/"+chasName+"/Sensors/Fans/Fan.Slot"),
				psModel, "Fan/"+fanName, ch, eb, ew)
			updateFns = append(updateFns, fanController.ConfigChangedFn)

			p := &domain.RedfishResourceProperty{}
			p.Parse(fan)
			fan_views = append(fan_views, p)
		}

		fanView := &domain.RedfishResourceProperty{Value: fan_views}
		thermalModel.ApplyOption(model.UpdateProperty("fan_views", fanView))
	*/

	// ************************************************************************
	// CHASSIS IOM.Slot
	// ************************************************************************
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

		iomController, _ := ar_mapper.NewARMappingController(ctx,
			logger.New("module", "Chassis/"+iomName, "module", "Chassis/IOM.Slot"),
			iom, "Managers/"+iomName, ch, eb, ew)
		updateFns = append(updateFns, iomController.ConfigChangedFn)

		/*
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
		*/

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
		sledController, _ := ar_mapper.NewARMappingController(ctx, sledLogger, sled, "Chassis/"+sledName, ch, eb, ew)
		updateFns = append(updateFns, sledController.ConfigChangedFn)

		/*
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
		*/

	}

	// VIPER Config:
	// pull the config from the YAML file to populate some static config options
	self.configChangeHandler = func() {
		logger.Info("Re-applying configuration from config file.")
		self.sessionModel.ApplyOption(model.UpdateProperty("session_timeout", cfgMgr.GetInt("session.timeout")))

		for _, fn := range updateFns {
			fn(ctx, cfgMgr)
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

	self.sessionModel.AddPropertyObserver("session_timeout", func(newval interface{}) {
		viperMu.Lock()
		cfgMgr.Set("session.timeout", newval.(int))
		viperMu.Unlock()
		dumpViperConfig()
	})

	return self
}
