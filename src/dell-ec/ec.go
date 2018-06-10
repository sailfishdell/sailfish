// Build tags: only build this for the simulation build. Be sure to note the required blank line after.
// +build ec

package obmc

import (
	"context"
	"sync"

	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	"github.com/superchalupa/go-redfish/src/ocp/root"
	"github.com/superchalupa/go-redfish/src/ocp/session"
	"github.com/superchalupa/go-redfish/src/ocp/stdcollections"
	"github.com/superchalupa/go-redfish/src/ocp/view"

	"github.com/superchalupa/go-redfish/src/dell-resources/ar_mapper"
	"github.com/superchalupa/go-redfish/src/dell-resources/attributes"
	chasCMCIntegrated "github.com/superchalupa/go-redfish/src/dell-resources/chassis/cmc.integrated"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/iom.slot"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/system.chassis"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/system.chassis/power"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/system.chassis/power/powersupply"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/system.chassis/thermal"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/system.chassis/thermal/fans"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/system.modular"
	mgrCMCIntegrated "github.com/superchalupa/go-redfish/src/dell-resources/managers/cmc.integrated"
	"github.com/superchalupa/go-redfish/src/dell-resources/test"
	"github.com/superchalupa/go-redfish/src/dell-resources/update_service"
)

type ocp struct {
	configChangeHandler func()
}

func (o *ocp) ConfigChangeHandler() { o.configChangeHandler() }

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.Mutex, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) *ocp {
	logger = logger.New("module", "ec")
	self := &ocp{}

	updateFns := []func(context.Context, *viper.Viper){}
	swinvViews := []*view.View{}

	//
	// Create the (empty) model behind the /redfish/v1 service root. Nothing interesting here
	//
	// No Logger
	// No Model
	// No Controllers
	// View created so we have a place to hold the aggregate UUID and URI
	rootView := view.New(
		view.WithURI("/redfish/v1"),
	)
	domain.RegisterPlugin(func() domain.Plugin { return rootView })
	root.AddAggregate(ctx, rootView, ch, eb, ew)

	//*********************************************************************
	//  /redfish/v1/testview - a proof of concept test view and example
	//*********************************************************************
	// construction order:
	//   1) model
	//   2) controller(s) - pass model by args
	//   3) views - pass models and controllers by args
	//   4) aggregate - pass view
	//
	testLogger := logger.New("module", "testview")
	testModel := model.New(
		model.UpdateProperty("unique_name", "test_unique_name"),
		model.UpdateProperty("name", "name"),
		model.UpdateProperty("description", "description"),
	)

	armapper, _ := ar_mapper.New(ctx, testLogger, testModel, "test/testview", "CMC.Integrated.1", ch, eb, ew)
	updateFns = append(updateFns, armapper.ConfigChangedFn)
	testView := view.New(
		view.WithModel("default", testModel),
		view.WithController("ar_mapper", armapper),
		view.WithURI(rootView.GetURI()+"/testview"),
	)
	domain.RegisterPlugin(func() domain.Plugin { return testView })
	test.AddAggregate(ctx, testView, ch)

	//*********************************************************************
	//  /redfish/v1/{Managers,Chassis,Systems,Accounts}
	//*********************************************************************
	// Add standard collections: Systems, Chassis, Mangers, Accounts
	//
	stdcollections.AddAggregate(ctx, rootView.GetUUID(), rootView.GetURI(), ch)

	//*********************************************************************
	// /redfish/v1/Sessions
	//*********************************************************************
	//
	sessionLogger := logger.New("module", "SessionService")
	sessionModel := model.New(
		model.UpdateProperty("session_timeout", 30))
	// the controller is what updates the model when ar entries change, also
	// handles patch from redfish
	armapper, _ = ar_mapper.New(ctx, sessionLogger, sessionModel, "SessionService", "", ch, eb, ew)
	updateFns = append(updateFns, armapper.ConfigChangedFn)
	sessionView := view.New(
		view.WithModel("default", sessionModel),
		view.WithController("ar_mapper", armapper),
		view.WithURI(rootView.GetURI()+"/SessionService"))
	domain.RegisterPlugin(func() domain.Plugin { return sessionView })
	session.AddAggregate(ctx, sessionView, rootView.GetUUID(), ch, eb, ew)

	//
	// Loop to create similarly named manager objects and the things attached there.
	//
	mgrLogger := logger.New("module", "Managers")
	var managers []*view.View
	for _, mgrName := range []string{
		"CMC.Integrated.1",
		"CMC.Integrated.2",
	} {
		//*********************************************************************
		// /redfish/v1/Managers/CMC.Integrated
		//*********************************************************************
		mgrLogger := mgrLogger.New("module", "Managers/"+mgrName, "module", "Managers/CMC.Integrated")
		mdl := model.New(
			mgrCMCIntegrated.WithUniqueName(mgrName),
			model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),
		)

		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		fwmapper, _ := ar_mapper.New(ctx, mgrLogger.New("module", "firmware/inventory"), mdl, "firmware/inventory", mgrName, ch, eb, ew)
		// need to have a separate model to hold fpga ver
		//fpgamapper, _ := ar_mapper.New(ctx, mgrLogger, mdl, "fpga_inventory", mgrName, ch, eb, ew)
		armapper, _ := ar_mapper.New(ctx, mgrLogger, mdl, "Managers/CMC.Integrated", mgrName, ch, eb, ew)
		updateFns = append(updateFns, armapper.ConfigChangedFn)
		updateFns = append(updateFns, fwmapper.ConfigChangedFn)

		// This controller will populate 'attributes' property with AR entries matching this FQDD ('mgrName')
		ardumper, _ := attributes.NewController(ctx, mdl, []string{mgrName}, ch, eb, ew)

		vw := view.New(
			view.WithURI(rootView.GetURI()+"/Managers/"+mgrName),
			view.WithModel("default", mdl),
			view.WithModel("swinv", mdl), // common name for swinv model, shared in this case
			view.WithController("ar_mapper", armapper),
			view.WithController("ar_dump", ardumper),
			view.WithController("fw_mapper", fwmapper),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
		)
		domain.RegisterPlugin(func() domain.Plugin { return vw })
		managers = append(managers, vw)
		swinvViews = append(swinvViews, vw)

		// add the aggregate to the view tree
		mgrCMCIntegrated.AddAggregate(ctx, mgrLogger, vw, ch, eb, ew)
		attributes.AddAggregate(ctx, vw, rootView.GetURI()+"/Managers/"+mgrName+"/Attributes", ch)

		//*********************************************************************
		// Create CHASSIS objects for CMC.Integrated.N
		//*********************************************************************
		chasLogger := logger.New("module", "Chassis/"+mgrName, "module", "Chassis/CMC.Integrated")
		chasModel, _ := mgrCMCIntegrated.New(
			mgrCMCIntegrated.WithUniqueName(mgrName),
			model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),
		)
		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish... re-use the same mappings from Managers
		armapper, _ = ar_mapper.New(ctx, chasLogger, chasModel, "Managers/CMC.Integrated", mgrName, ch, eb, ew)
		updateFns = append(updateFns, armapper.ConfigChangedFn)

		// This controller will populate 'attributes' property with AR entries matching this FQDD ('mgrName')
		ardumper, _ = attributes.NewController(ctx, chasModel, []string{mgrName}, ch, eb, ew)

		vw = view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+mgrName),
			view.WithModel("default", chasModel),
			view.WithController("ar_mapper", armapper),
			view.WithController("ar_dump", ardumper),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
		)
		domain.RegisterPlugin(func() domain.Plugin { return vw })

		// add the aggregate to the view tree
		chasCMCIntegrated.AddAggregate(ctx, chasLogger, vw, ch)
		attributes.AddAggregate(ctx, vw, rootView.GetURI()+"/Chassis/"+mgrName+"/Attributes", ch)
	}

	chasName := "System.Chassis.1"
	chasLogger := logger.New("module", "Chassis/"+chasName, "module", "Chassis/System.Chassis")
	{
		// ************************************************************************
		// CHASSIS System.Chassis.1
		// ************************************************************************
		chasModel := model.New(
			model.UpdateProperty("unique_name", chasName),
			model.UpdateProperty("managed_by", []map[string]string{{"@odata.id": managers[0].GetURI()}}),
			model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),
		)
		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		armapper, _ := ar_mapper.New(ctx, chasLogger, chasModel, "Chassis/System.Chassis", chasName, ch, eb, ew)
		updateFns = append(updateFns, armapper.ConfigChangedFn)

		// This controller will populate 'attributes' property with AR entries matching this FQDD ('chasName')
		ardumper, _ := attributes.NewController(ctx, chasModel, []string{chasName}, ch, eb, ew)

		vw := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+chasName),
			view.WithModel("default", chasModel),
			view.WithController("ar_mapper", armapper),
			view.WithController("ar_dump", ardumper),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
		)
		domain.RegisterPlugin(func() domain.Plugin { return vw })

		// Create the .../Attributes URI. Attributes are stored in the attributes property of the chasModel
		system_chassis.AddAggregate(ctx, chasLogger, vw, ch, eb, ew)
		attributes.AddAggregate(ctx, vw, rootView.GetURI()+"/Chassis/"+chasName+"/Attributes", ch)
	}

	//*********************************************************************
	// Create Power objects for System.Chassis.1
	//*********************************************************************
	powerLogger := chasLogger.New("module", "Chassis/System.Chassis/Power")

	powerModel := model.New(
		mgrCMCIntegrated.WithUniqueName("Power"),
		model.UpdateProperty("power_supply_views", []interface{}{}),
	)
	// the controller is what updates the model when ar entries change,
	// also handles patch from redfish
	armapper, _ = ar_mapper.New(ctx, powerLogger, powerModel, "Chassis/System.Chassis/Power", chasName, ch, eb, ew)
	updateFns = append(updateFns, armapper.ConfigChangedFn)

	vw := view.New(
		view.WithURI(rootView.GetURI()+"/Chassis/"+chasName+"/Power"),
		view.WithModel("default", powerModel),
		view.WithController("ar_mapper", armapper),
	)
	domain.RegisterPlugin(func() domain.Plugin { return vw })
	power.AddAggregate(ctx, powerLogger, vw, ch)

	psu_views := []interface{}{}
	for _, psuName := range []string{
		"PSU.Slot.1", "PSU.Slot.2", "PSU.Slot.3",
		"PSU.Slot.4", "PSU.Slot.5", "PSU.Slot.6",
	} {
		psuLogger := powerLogger.New("module", "Chassis/System.Chassis/Power/PowerSupply")

		psuModel := model.New(
			model.UpdateProperty("unique_name", psuName),
			model.UpdateProperty("unique_id", psuName),
			model.UpdateProperty("name", psuName),
			model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),
		)
		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		armapper, _ := ar_mapper.New(ctx, psuLogger, psuModel, "PowerSupply/PSU.Slot", psuName, ch, eb, ew)
		updateFns = append(updateFns, armapper.ConfigChangedFn)

		// This controller will populate 'attributes' property with AR entries matching this FQDD ('psuName')
		ardumper, _ := attributes.NewController(ctx, psuModel, []string{psuName}, ch, eb, ew)

		vw := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+chasName+"/Power/PowerSupplies/"+psuName),
			view.WithModel("default", powerModel),
			view.WithController("ar_mapper", armapper),
			view.WithController("ar_dumper", ardumper),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
		)
		domain.RegisterPlugin(func() domain.Plugin { return vw })

		psu := powersupply.AddAggregate(ctx, psuLogger, vw, ch)

		p := &domain.RedfishResourceProperty{}
		p.Parse(psu)
		psu_views = append(psu_views, p)
	}
	powerModel.ApplyOption(model.UpdateProperty("power_supply_views", &domain.RedfishResourceProperty{Value: psu_views}))

	//*********************************************************************
	// Create Thermal objects for System.Chassis.1
	//*********************************************************************
	thermalLogger := chasLogger.New("module", "Chassis/System.Chassis/Thermal")

	thermalModel := model.New(
		mgrCMCIntegrated.WithUniqueName("Thermal"),
		model.UpdateProperty("fan_views", []interface{}{}),
		model.UpdateProperty("thermal_views", []interface{}{}),
	)
	// the controller is what updates the model when ar entries change,
	// also handles patch from redfish
	armapper, _ = ar_mapper.New(ctx, thermalLogger, thermalModel, "Chassis/System.Chassis/Thermal", chasName, ch, eb, ew)
	updateFns = append(updateFns, armapper.ConfigChangedFn)

	thermalView := view.New(
		view.WithURI(rootView.GetURI()+"/Chassis/"+chasName+"/Thermal"),
		view.WithModel("default", thermalModel),
		view.WithController("ar_mapper", armapper),
	)
	domain.RegisterPlugin(func() domain.Plugin { return thermalView })
	thermal.AddAggregate(ctx, thermalLogger, thermalView, ch)

	fan_views := []interface{}{}
	for _, fanName := range []string{
		"Fan.Slot.1", "Fan.Slot.2", "Fan.Slot.3",
		"Fan.Slot.4", "Fan.Slot.5", "Fan.Slot.6",
		"Fan.Slot.7", "Fan.Slot.8", "Fan.Slot.9",
	} {
		fanLogger := thermalLogger.New("module", "Chassis/System.Chassis/Thermal/Fan")

		fanModel := model.New(
			model.UpdateProperty("unique_id", fanName),
			model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),
		)
		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		armapper, _ := ar_mapper.New(ctx, fanLogger, fanModel, "Fans/Fan.Slot", fanName, ch, eb, ew)
		updateFns = append(updateFns, armapper.ConfigChangedFn)

		// This controller will populate 'attributes' property with AR entries matching this FQDD ('fanName')
		ardumper, _ := attributes.NewController(ctx, fanModel, []string{fanName}, ch, eb, ew)

		v := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+chasName+"/Sensors/Fans/"+fanName),
			view.WithModel("default", fanModel),
			view.WithController("ar_mapper", armapper),
			view.WithController("ar_dumper", ardumper),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
		)
		domain.RegisterPlugin(func() domain.Plugin { return v })
		fanFragment := fans.AddAggregate(ctx, fanLogger, v, ch)

		p := &domain.RedfishResourceProperty{}
		p.Parse(fanFragment)
		fan_views = append(fan_views, p)
	}
	thermalModel.ApplyOption(model.UpdateProperty("fan_views", &domain.RedfishResourceProperty{Value: fan_views}))

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
		iomLogger := chasLogger.New("module", "Chassis/"+iomName, "module", "Chassis/IOM.Slot")
		iomModel := model.New(
			model.UpdateProperty("unique_name", iomName),
			model.UpdateProperty("managed_by", []map[string]string{{"@odata.id": managers[0].GetURI()}}),
		)
		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		armapper, _ := ar_mapper.New(ctx, iomLogger, iomModel, "Chassis/IOM.Slot", iomName, ch, eb, ew)
		updateFns = append(updateFns, armapper.ConfigChangedFn)

		// This controller will populate 'attributes' property with AR entries matching this FQDD ('iomName')
		ardumper, _ := attributes.NewController(ctx, iomModel, []string{iomName}, ch, eb, ew)

		iomView := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+iomName),
			view.WithModel("default", iomModel),
			view.WithController("ar_mapper", armapper),
			view.WithController("ar_dumper", ardumper),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
		)
		domain.RegisterPlugin(func() domain.Plugin { return iomView })
		iom_chassis.AddAggregate(ctx, iomLogger, iomView, ch, eb, ew)
		attributes.AddAggregate(ctx, iomView, rootView.GetURI()+"/Chassis/"+iomName+"/Attributes", ch)
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
		sledLogger := chasLogger.New("module", "Chassis/System.Modular", "module", "Chassis/"+sledName)
		sledModel := model.New(
			model.UpdateProperty("unique_name", sledName),
			model.UpdateProperty("managed_by", []map[string]string{{"@odata.id": managers[0].GetURI()}}),
		)
		armapper, _ := ar_mapper.New(ctx, sledLogger, sledModel, "Chassis/System.Modular", sledName, ch, eb, ew)
		updateFns = append(updateFns, armapper.ConfigChangedFn)

		// This controller will populate 'attributes' property with AR entries matching this FQDD ('sledName')
		ardumper, _ := attributes.NewController(ctx, sledModel, []string{sledName}, ch, eb, ew)

		sledView := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+sledName),
			view.WithModel("default", sledModel),
			view.WithController("ar_mapper", armapper),
			view.WithController("ar_dumper", ardumper),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
		)
		domain.RegisterPlugin(func() domain.Plugin { return sledView })
		sled_chassis.AddAggregate(ctx, sledLogger, sledView, ch, eb, ew)
		attributes.AddAggregate(ctx, sledView, rootView.GetURI()+"/Chassis/"+sledName+"/Attributes", ch)
	}

	// Software inventory
	invModels := map[string]*model.Model{}
	for _, invName := range []string{
		"PSU.Slot.1", "PSU.Slot.2", "PSU.Slot.3",
		"PSU.Slot.4", "PSU.Slot.5", "PSU.Slot.6",
	} {
		invLogger := logger.New("module", "UpdateService")
		invModel := model.New(
			model.UpdateProperty("unique_name", invName),
		)
		armapper, _ := ar_mapper.New(ctx, invLogger, invModel, "UpdateService", "", ch, eb, ew)
		updateFns = append(updateFns, armapper.ConfigChangedFn)
		invModels[invName] = invModel
	}

	{
		updsvcLogger := logger.New("module", "UpdateService")
		mdl := model.New()

		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		armapper, _ := ar_mapper.New(ctx, updsvcLogger, mdl, "update_service", "", ch, eb, ew)
		updateFns = append(updateFns, armapper.ConfigChangedFn)

		vw := view.New(
			view.WithURI(rootView.GetURI()+"/UpdateService"),
			view.WithModel("default", mdl),
			view.WithController("ar_mapper", armapper),
		)
		domain.RegisterPlugin(func() domain.Plugin { return vw })

		// add the aggregate to the view tree
		update_service.AddAggregate(ctx, rootView, vw, ch)
		update_service.EnhanceAggregate(ctx, vw, rootView, ch)
	}

	sw := map[string]map[string]*model.Model{}
	swMu := sync.Mutex{}

	obsLogger := logger.New("module", "observer")
	fn := func(mdl *model.Model, property string, oldValue, newValue interface{}) {
		// model is locked when we enter observer
		obsLogger.Debug("MODEL POPERTY CHANGE", "model", mdl, "property", property, "oldValue", oldValue, "newValue", newValue)

		classRaw, ok := mdl.GetPropertyOkUnlocked("device_class")
		if !ok || classRaw == nil {
			obsLogger.Debug("DID NOT GET device_class raw")
			return
		}

		class, ok := classRaw.(string)
		if !ok {
			obsLogger.Debug("DID NOT GET class string")
			return
		}

		versionRaw, ok := mdl.GetPropertyOkUnlocked("version")
		if !ok || versionRaw == nil {
			obsLogger.Debug("DID NOT GET version raw")
			return
		}

		version, ok := versionRaw.(string)
		if !ok {
			obsLogger.Debug("DID NOT GET version string")
			return
		}

		obsLogger.Info("GOT FULL SWVERSION INFO", "model", mdl, "property", property, "oldValue", oldValue, "newValue", newValue)
		swMu.Lock()
		defer swMu.Unlock()
		ver, ok := sw[class]
		if !ok {
			ver = map[string]*model.Model{}
		}
		ver[version] = mdl
		sw[class] = ver

		// TODO: delete any old copies of this model in the tree
	}

	// Set up observers for each swinv model
	for _, swinvView := range swinvViews {
		// going to assume each view has swinv model at 'swinv'
		mdl := swinvView.GetModel("swinv")
		mdl.AddObserver("swinv", fn)
	}

	// VIPER Config:
	// pull the config from the YAML file to populate some static config options
	self.configChangeHandler = func() {
		logger.Info("Re-applying configuration from config file.")
		sessionModel.ApplyOption(model.UpdateProperty("session_timeout", cfgMgr.GetInt("session.timeout")))

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

	sessObsLogger := logger.New("module", "observer")
	sessionModel.AddObserver("viper", func(m *model.Model, property string, oldValue, newValue interface{}) {
		sessObsLogger.Info("Session variable changed", "model", m, "property", property, "oldValue", oldValue, "newValue", newValue)
		if property == "session_timeout" {
			viperMu.Lock()
			cfgMgr.Set("session.timeout", newValue.(int))
			viperMu.Unlock()
			dumpViperConfig()
		}
	})

	return self
}
