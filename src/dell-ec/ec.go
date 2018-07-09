package dell_ec

import (
	"context"
	"sync"

	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"

	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	"github.com/superchalupa/go-redfish/src/actionhandler"
	"github.com/superchalupa/go-redfish/src/eventwaiter"
	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/eventservice"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	"github.com/superchalupa/go-redfish/src/ocp/root"
	"github.com/superchalupa/go-redfish/src/ocp/session"
	"github.com/superchalupa/go-redfish/src/ocp/static_mapper"
	"github.com/superchalupa/go-redfish/src/ocp/stdcollections"
	"github.com/superchalupa/go-redfish/src/ocp/telemetryservice"
	"github.com/superchalupa/go-redfish/src/ocp/test_aggregate"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	"github.com/superchalupa/go-redfish/src/ocp/event"
	"github.com/superchalupa/go-redfish/src/ocp/awesome_mapper"

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
	"github.com/superchalupa/go-redfish/src/dell-resources/fan_controller"
	"github.com/superchalupa/go-redfish/src/dell-resources/health_mapper"
	mgrCMCIntegrated "github.com/superchalupa/go-redfish/src/dell-resources/managers/cmc.integrated"
	"github.com/superchalupa/go-redfish/src/dell-resources/registries"
	"github.com/superchalupa/go-redfish/src/dell-resources/registries/registry"
	"github.com/superchalupa/go-redfish/src/dell-resources/update_service"
	"github.com/superchalupa/go-redfish/src/dell-resources/update_service/firmware_inventory"

	ah "github.com/superchalupa/go-redfish/src/actionhandler"
)

type ocp struct {
	configChangeHandler func()
}

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
}

func (o *ocp) ConfigChangeHandler() { o.configChangeHandler() }

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.Mutex, ch eh.CommandHandler, eb eh.EventBus, ew waiter) *ocp {
	logger = logger.New("module", "ec")
	self := &ocp{}

	updateFns := []func(context.Context, *viper.Viper){}
	swinvViews := []*view.View{}

	// These three all set up a waiter for the root service to appear, so init root service after.
	actionhandler.Setup(ctx, ch, eb, ew)
	eventservice.Setup(ctx, ch, eb)
	telemetryservice.Setup(ctx, ch, eb)
	health_mapper.Setup(ctx, ch, eb)
	fan_controller.Setup(ctx, ch, eb)
    event.Setup(ch, eb)

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
	root.AddAggregate(ctx, rootView, ch, eb)

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
    awesome_mapper.New(ctx, testLogger, cfgMgr, testModel, "testcontroller", map[string]interface{}{"fqdd": "System.Modular.1"})

	armapper, _ := ar_mapper.New(ctx, testLogger, testModel, "test/testview", "CMC.Integrated.1", ch, eb, ew)
	updateFns = append(updateFns, armapper.ConfigChangedFn)
	testView := view.New(
		view.WithModel("default", testModel),
		view.WithController("ar_mapper", armapper),
		view.WithURI(rootView.GetURI()+"/testview"),
		eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
	)
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
		view.WithURI(rootView.GetURI()+"/SessionService"),
		eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
	)
	session.AddAggregate(ctx, sessionView, rootView.GetUUID(), ch, eb, ew)

	//*********************************************************************
	// /redfish/v1/EventService
	// /redfish/v1/TelemetryService
	//*********************************************************************
	eventservice.StartEventService(ctx, logger, rootView)
	telemetryservice.StartTelemetryService(ctx, logger, rootView)
	// TODO: this guy returns a view we can use if we want to hook up a controller

	//*********************************************************************
	// /redfish/v1/Registries
	//*********************************************************************
	registryLogger := logger.New("module", "Registries")
	registryModel := model.New()

	// static config controller, initlize values based on yaml config
	staticMapper, _ := static_mapper.New(ctx, registryLogger, registryModel, "Registries")
	updateFns = append(updateFns, staticMapper.ConfigChangedFn)

	registryView := view.New(
		view.WithURI(rootView.GetURI()+"/Registries"),
		view.WithModel("default", registryModel),
	)
	registries.AddAggregate(ctx, registryLogger, registryView, ch, eb)

	registry_views := []interface{}{}
	for _, registryNames := range []string{
		"Messages", "BaseMessages", "ManagerAttributeRegistry",
	} {
		regModel := model.New(
			model.UpdateProperty("registry_id", registryNames),
		)

		// static config controller, initlize values based on yaml config
		staticMapper, _ := static_mapper.New(ctx, registryLogger, regModel, "Registries/"+registryNames)
		updateFns = append(updateFns, staticMapper.ConfigChangedFn)

		rv := view.New(
			view.WithURI(rootView.GetURI()+"/Registries/"+registryNames),
			view.WithModel("default", regModel),
		)
		registry.AddAggregate(ctx, registryLogger, rv, ch, eb)
	}
	registryModel.ApplyOption(model.UpdateProperty("registry_views", &domain.RedfishResourceProperty{Value: registry_views}))

	//HEALTH
	// The following model maps a bunch of health related stuff that can be tracked once at a global level.
	// we can add this model to the views that need to expose it
	globalHealthModel := model.New()
	healthLogger := logger.New("module", "health_rollup")
	health_mapper.New(healthLogger, globalHealthModel, "fan_rollup", "System.Chassis.1#SubSystem.1#Fan")
	health_mapper.New(healthLogger, globalHealthModel, "temperature_rollup", "System.Chassis.1#SubSystem.1#Temperature")
	health_mapper.New(healthLogger, globalHealthModel, "mm_rollup", "System.Chassis.1#SubSystem.1#MM")
	health_mapper.New(healthLogger, globalHealthModel, "sled_rollup", "System.Chassis.1#SubSystem.1#SledSystem")
	health_mapper.New(healthLogger, globalHealthModel, "psu_rollup", "System.Chassis.1#SubSystem.1#PowerSupply")
	health_mapper.New(healthLogger, globalHealthModel, "cmc_rollup", "System.Chassis.1#SubSystem.1#CMC")
	health_mapper.New(healthLogger, globalHealthModel, "misc_rollup", "System.Chassis.1#SubSystem.1#Miscellaneous")
	health_mapper.New(healthLogger, globalHealthModel, "battery_rollup", "System.Chassis.1#SubSystem.1#Battery")
	health_mapper.New(healthLogger, globalHealthModel, "iom_rollup", "System.Chassis.1#SubSystem.1#IOMSubsystem")

	//
	// Loop to create similarly named manager objects and the things attached there.
	//
	mgrLogger := logger.New("module", "Managers")
	var managers []*view.View
	mgrRedundancyMdl := model.New(
		model.UpdateProperty("health", "red TEST health"),
		model.UpdateProperty("state", "red TEST state"),
		model.UpdateProperty("health_rollup", "red TEST"),
	)
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

			// manually add health properties until we get a mapper to automatically manage these
			model.UpdateProperty("health", "TEST health"),
			model.UpdateProperty("state", "TEST state"),
			model.UpdateProperty("health_rollup", "TEST"),
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

		mgrCmcVw := view.New(
			view.WithURI(rootView.GetURI()+"/Managers/"+mgrName),
			view.WithModel("redundancy_health", mgrRedundancyMdl), // health info in default model
			view.WithModel("health", mdl),                         // health info in default model
			view.WithModel("swinv", mdl),                          // common name for swinv model, shared in this case
			view.WithModel("default", mdl),
			view.WithController("ar_mapper", armapper),
			view.WithController("ar_dump", ardumper),
			view.WithController("fw_mapper", fwmapper),

			ah.WithAction(ctx, mgrLogger, "manager.reset", "/Actions/ManagerReset", bmcReset, ch, eb),
			ah.WithAction(ctx, mgrLogger, "manager.resettodefaults", "/Actions/ResetToDefaults", bmcResetToDefaults, ch, eb),

			ah.WithAction(ctx, mgrLogger, "manager.forcefailover", "/Actions/ForceFailover", bmcForceFailover, ch, eb),
			ah.WithAction(ctx, mgrLogger, "manager.exportsystemconfig", "/Actions/ExportSystemConfig", exportSystemConfiguration, ch, eb),
			ah.WithAction(ctx, mgrLogger, "manager.importsystemconfig", "/Actions/ImportSystemConfig", importSystemConfiguration, ch, eb),
			ah.WithAction(ctx, mgrLogger, "manager.importsystemconfigpreview", "/Actions/ImportSystemConfigPreview", importSystemConfigurationPreview, ch, eb),

			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
			eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
		)

		managers = append(managers, mgrCmcVw)
		swinvViews = append(swinvViews, mgrCmcVw)

		// add the aggregate to the view tree
		mgrCMCIntegrated.AddAggregate(ctx, mgrLogger, mgrCmcVw, ch)
		attributes.AddAggregate(ctx, mgrCmcVw, rootView.GetURI()+"/Managers/"+mgrName+"/Attributes", ch)

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

		chasCmcVw := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+mgrName),
			view.WithModel("default", chasModel),
			view.WithController("ar_mapper", armapper),
			view.WithController("ar_dump", ardumper),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
			eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
		)

		// add the aggregate to the view tree
		chasCMCIntegrated.AddAggregate(ctx, chasLogger, chasCmcVw, ch)
		attributes.AddAggregate(ctx, chasCmcVw, rootView.GetURI()+"/Chassis/"+mgrName+"/Attributes", ch)
	}

	chasLogger := logger.New("module", "Chassis")
	{
		// ************************************************************************
		// CHASSIS System.Chassis.1
		// ************************************************************************
		chasName := "System.Chassis.1"
		sysChasLogger := chasLogger.New("module", "Chassis/"+chasName, "module", "Chassis/System.Chassis")
		chasModel := model.New(
			model.UpdateProperty("unique_name", chasName),
			model.UpdateProperty("managed_by", []map[string]string{{"@odata.id": managers[0].GetURI()}}),
			model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),
		)
		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		armapper, _ := ar_mapper.New(ctx, sysChasLogger, chasModel, "Chassis/System.Chassis", chasName, ch, eb, ew)
		updateFns = append(updateFns, armapper.ConfigChangedFn)

		// This controller will populate 'attributes' property with AR entries matching this FQDD ('chasName')
		ardumper, _ := attributes.NewController(ctx, chasModel, []string{chasName}, ch, eb, ew)

		sysChasVw := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+chasName),
			view.WithModel("default", chasModel),
			view.WithController("ar_mapper", armapper),
			view.WithController("ar_dump", ardumper),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
			ah.WithAction(ctx, sysChasLogger, "chassis.reset", "/Actions/Chassis.Reset", chassisReset, ch, eb),
			ah.WithAction(ctx, sysChasLogger, "msmconfigbackup", "/Actions/Oem/MSMConfigBackup", msmConfigBackup, ch, eb),
			ah.WithAction(ctx, sysChasLogger, "chassis.msmconfigbackup", "/Actions/Oem/DellChassis.MSMConfigBackup", chassisMSMConfigBackup, ch, eb),
			eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
		)

		// Create the .../Attributes URI. Attributes are stored in the attributes property of the chasModel
		system_chassis.AddAggregate(ctx, sysChasLogger, sysChasVw, ch, eb, ew)
		attributes.AddAggregate(ctx, sysChasVw, rootView.GetURI()+"/Chassis/"+chasName+"/Attributes", ch)

		//*********************************************************************
		// Create Power objects for System.Chassis.1
		//*********************************************************************
		powerLogger := sysChasLogger.New("module", "Chassis/System.Chassis/Power")

		powerModel := model.New(
			mgrCMCIntegrated.WithUniqueName("Power"),
			model.UpdateProperty("power_supply_views", []interface{}{}),
		)
		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		armapper, _ = ar_mapper.New(ctx, powerLogger, powerModel, "Chassis/System.Chassis/Power", chasName, ch, eb, ew)
		updateFns = append(updateFns, armapper.ConfigChangedFn)

		sysChasPwrVw := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+chasName+"/Power"),
			view.WithModel("default", powerModel),
			view.WithController("ar_mapper", armapper),
			eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
		)
		power.AddAggregate(ctx, powerLogger, sysChasPwrVw, ch)

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
			fwmapper, _ := ar_mapper.New(ctx, psuLogger.New("module", "firmware/inventory"), psuModel, "firmware/inventory", psuName, ch, eb, ew)
			// the controller is what updates the model when ar entries change,
			// also handles patch from redfish
			armapper, _ := ar_mapper.New(ctx, psuLogger, psuModel, "PowerSupply/PSU.Slot", psuName, ch, eb, ew)
			updateFns = append(updateFns, armapper.ConfigChangedFn)
			updateFns = append(updateFns, fwmapper.ConfigChangedFn)

			// This controller will populate 'attributes' property with AR entries matching this FQDD ('psuName')
			ardumper, _ := attributes.NewController(ctx, psuModel, []string{psuName}, ch, eb, ew)

			sysChasPwrPsuVw := view.New(
				view.WithURI(rootView.GetURI()+"/Chassis/"+chasName+"/Power/PowerSupplies/"+psuName),
				view.WithModel("default", powerModel),
				view.WithModel("swinv", powerModel),
				view.WithController("ar_mapper", armapper),
				view.WithController("ar_dumper", ardumper),
				view.WithController("fw_mapper", fwmapper),
				view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
				eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
			)
			swinvViews = append(swinvViews, sysChasPwrPsuVw)

			psu := powersupply.AddAggregate(ctx, psuLogger, sysChasPwrPsuVw, ch)

			p := &domain.RedfishResourceProperty{}
			p.Parse(psu)
			psu_views = append(psu_views, p)
		}
		powerModel.ApplyOption(model.UpdateProperty("power_supply_views", &domain.RedfishResourceProperty{Value: psu_views}))

		//*********************************************************************
		// Create Thermal objects for System.Chassis.1
		//*********************************************************************
		thermalLogger := sysChasLogger.New("module", "Chassis/System.Chassis/Thermal")

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
			eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
		)
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
			fwmapper, _ := ar_mapper.New(ctx, fanLogger.New("module", "firmware/inventory"), fanModel, "firmware/inventory", fanName, ch, eb, ew)
			updateFns = append(updateFns, fwmapper.ConfigChangedFn)
			// the controller is what updates the model when ar entries change,
			// also handles patch from redfish
			armapper, _ := ar_mapper.New(ctx, fanLogger, fanModel, "Fans/Fan.Slot", fanName, ch, eb, ew)
			updateFns = append(updateFns, armapper.ConfigChangedFn)

			fan_controller.New(fanLogger, fanModel, "System.Chassis.1#"+fanName)

			// This controller will populate 'attributes' property with AR entries matching this FQDD ('fanName')
			ardumper, _ := attributes.NewController(ctx, fanModel, []string{fanName}, ch, eb, ew)

			v := view.New(
				view.WithURI(rootView.GetURI()+"/Chassis/"+chasName+"/Sensors/Fans/"+fanName),
				view.WithModel("default", fanModel),
				view.WithModel("swinv", fanModel),
				view.WithController("ar_mapper", armapper),
				view.WithController("ar_dumper", ardumper),
				view.WithController("fw_mapper", fwmapper),
				view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
				eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
			)
			swinvViews = append(swinvViews, v)

			fanFragment := fans.AddAggregate(ctx, fanLogger, v, ch)

			p := &domain.RedfishResourceProperty{}
			p.Parse(fanFragment)
			fan_views = append(fan_views, p)
		}
		thermalModel.ApplyOption(model.UpdateProperty("fan_views", &domain.RedfishResourceProperty{Value: fan_views}))
	}

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
		fwmapper, _ := ar_mapper.New(ctx, iomLogger.New("module", "firmware/inventory"), iomModel, "firmware/inventory", iomName, ch, eb, ew)
		updateFns = append(updateFns, fwmapper.ConfigChangedFn)
		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		armapper, _ := ar_mapper.New(ctx, iomLogger, iomModel, "Chassis/IOM.Slot", iomName, ch, eb, ew)
		updateFns = append(updateFns, armapper.ConfigChangedFn)

		// This controller will populate 'attributes' property with AR entries matching this FQDD ('iomName')
		ardumper, _ := attributes.NewController(ctx, iomModel, []string{iomName}, ch, eb, ew)

		//HEALTH
		health_mapper.New(iomLogger, iomModel, "health", "System.Chassis.1#SubSystem.1#"+iomName)

		iomView := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+iomName),
			view.WithModel("default", iomModel),
			view.WithModel("swinv", iomModel),
			view.WithController("ar_mapper", armapper),
			view.WithController("ar_dumper", ardumper),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
			ah.WithAction(ctx, iomLogger, "iom.chassis.reset", "/Actions/Chassis.Reset", iomChassisReset, ch, eb),
			ah.WithAction(ctx, iomLogger, "iom.resetpeakpowerconsumption", "/Actions/Oem/DellChassis.ResetPeakPowerConsumption", iomResetPeakPowerConsumption, ch, eb),
			ah.WithAction(ctx, iomLogger, "iom.virtualreseat", "/Actions/Oem/DellChassis.VirtualReseat", iomVirtualReseat, ch, eb),
			eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
		)
		swinvViews = append(swinvViews, iomView)
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

		//HEALTH
		health_mapper.New(sledLogger, sledModel, "health", "System.Chassis.1#SubSystem.1#"+sledName)

		sledView := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+sledName),
			view.WithModel("default", sledModel),
			view.WithController("ar_mapper", armapper),
			view.WithController("ar_dumper", ardumper),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
			ah.WithAction(ctx, sledLogger, "chassis.peripheralmapping", "/Actions/Oem/DellChassis.PeripheralMapping", chassisPeripheralMapping, ch, eb),
			ah.WithAction(ctx, sledLogger, "sledvirtualreseat", "/Actions/Chassis.VirtualReseat", sledVirtualReseat, ch, eb),
			ah.WithAction(ctx, sledLogger, "chassis.sledvirtualreseat", "/Actions/Oem/DellChassis.VirtualReseat", chassisSledVirtualReseat, ch, eb),
			eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
		)
		sled_chassis.AddAggregate(ctx, sledLogger, sledView, ch, eb)
		attributes.AddAggregate(ctx, sledView, rootView.GetURI()+"/Chassis/"+sledName+"/Attributes", ch)
	}

	{
		updsvcLogger := logger.New("module", "UpdateService")
		mdl := model.New()

		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		armapper, _ := ar_mapper.New(ctx, updsvcLogger, mdl, "update_service", "", ch, eb, ew)
		updateFns = append(updateFns, armapper.ConfigChangedFn)

		updSvcVw := view.New(
			view.WithURI(rootView.GetURI()+"/UpdateService"),
			view.WithModel("default", mdl),
			view.WithController("ar_mapper", armapper),
			ah.WithAction(ctx, updsvcLogger, "update.reset", "/Actions/Oem/DellUpdateService.Reset", updateReset, ch, eb),
			ah.WithAction(ctx, updsvcLogger, "update.eid674.reset", "/Actions/Oem/EID_674_UpdateService.Reset", updateEID674Reset, ch, eb),
			ah.WithAction(ctx, updsvcLogger, "update.syncup", "/Actions/Oem/DellUpdateService.Syncup", updateSyncup, ch, eb),
			ah.WithAction(ctx, updsvcLogger, "update.eid674.syncup", "/Actions/Oem/EID_674_UpdateService.Syncup", updateEID674Syncup, ch, eb),
			eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
		)

		// add the aggregate to the view tree
		update_service.AddAggregate(ctx, rootView, updSvcVw, ch)
		update_service.EnhanceAggregate(ctx, updSvcVw, rootView, ch)
	}

	//
	// Software Inventory
	//
	inv := map[string]*view.View{}
	model2View := map[*model.Model]*view.View{}
	swMu := sync.Mutex{}

	// Purpose of this function is to make a list of all of the firmware
	// inventory models and then create a reverse-mapping for the updateservice
	// firmwareinventory
	//
	// TODO: "QuickSync.Chassis.1"
	// TODO: "LCD.Chassis.1"
	// TODO: "ControlPanel.Chassis.1"
	// TODO: "CMC.Integrated.1" / "FPGAFWInventory"
	// TODO: "CMC.Integrated.2" / "FPGAFWInventory"
	//
	// DONE: "Fan.Slot.1"
	// DONE: "IOM.Slot.A1a"
	// DONE: "PSU.Slot.1"
	// DONE: "CMC.Integrated.1"

	obsLogger := logger.New("module", "observer")
	fn := func(mdl *model.Model, property string, newValue interface{}) {
		obsLogger.Info("observer entered", "model", mdl, "property", property, "newValue", newValue)

		classRaw, ok := mdl.GetPropertyOkUnlocked("fw_device_class")
		if !ok || classRaw == nil {
			obsLogger.Debug("DID NOT GET device_class raw")
			return
		}

		class, ok := classRaw.(string)
		if !ok || class == "" {
			obsLogger.Debug("DID NOT GET class string")
			return
		}

		versionRaw, ok := mdl.GetPropertyOkUnlocked("fw_version")
		if !ok || versionRaw == nil {
			obsLogger.Debug("DID NOT GET version raw")
			return
		}

		version, ok := versionRaw.(string)
		if !ok || version == "" {
			obsLogger.Debug("DID NOT GET version string")
			return
		}

		fqddRaw, ok := mdl.GetPropertyOkUnlocked("fw_fqdd")
		if !ok || fqddRaw == nil {
			obsLogger.Debug("DID NOT GET fqdd raw")
			return
		}

		fqdd, ok := fqddRaw.(string)
		if !ok || fqdd == "" {
			obsLogger.Debug("DID NOT GET fqdd string")
			return
		}

		swMu.Lock()
		defer swMu.Unlock()

		obsLogger.Info("GOT FULL SWVERSION INFO", "model", mdl, "property", property, "newValue", newValue)

		comp_ver_tuple := class + "-" + version

		fw_fqdd_list := []string{fqdd}
		fw_related_list := []map[string]interface{}{}

		invview, ok := inv[comp_ver_tuple]
		if !ok {
			// didn't previously have a view/model for this version/componentid, so make one
			//
			obsLogger.Info("No previous view, creating a new view.", "tuple", comp_ver_tuple)
			invmdl := model.New(
				model.UpdateProperty("fw_id", "Installed-"+comp_ver_tuple),
			)

			invview = view.New(
				view.WithURI(rootView.GetURI()+"/UpdateService/FirmwareInventory/Installed-"+comp_ver_tuple),
				view.WithModel("swinv", mdl),
				view.WithModel("firm", invmdl),
				eventservice.PublishResourceUpdatedEventsForModel(ctx, "swinv", eb),
				eventservice.PublishResourceUpdatedEventsForModel(ctx, "firm", eb),
			)
			inv[comp_ver_tuple] = invview
			firmware_inventory.AddAggregate(ctx, rootView, invview, ch)

			fw_related_list = append(fw_related_list, map[string]interface{}{"@odata.id": model2View[mdl].GetURI()})
		} else {
			obsLogger.Info("UPDATING previous view.", "tuple", comp_ver_tuple)

			// we have an existing view/model, so add our uniqueness to theirs
			firm_mdl := invview.GetModel("firm")
			if firm_mdl == nil {
				obsLogger.Info("Programming error: got an inventory view that doesn't have a 'firm' model. Should not be able to happen.", "tuple", comp_ver_tuple)
				return
			}

			raw_fqdd_list, ok := firm_mdl.GetPropertyOk("fw_fqdd_list")
			if !ok {
				raw_fqdd_list = []string{fqdd}
			}
			fw_fqdd_list, ok = raw_fqdd_list.([]string)
			if !ok {
				fw_fqdd_list = []string{fqdd}
			}
			obsLogger.Info("PREVIOUS FQDD LIST.", "fw_fqdd_list", fw_fqdd_list)

			add := true
			for _, m := range fw_fqdd_list {
				if m == fqdd {
					add = false
					break
				}
			}
			if add {
				fw_fqdd_list = append(fw_fqdd_list, fqdd)
			}

			raw_related_list, ok := firm_mdl.GetPropertyOk("fw_related_list")
			if !ok {
				raw_related_list = []map[string]interface{}{}
			}

			fw_related_list, ok = raw_related_list.([]map[string]interface{})
			if !ok {
				fw_related_list = []map[string]interface{}{}
			}

			obsLogger.Info("PREVIOUS related LIST.", "fw_related_list", fw_related_list)

			add = true
			for _, m := range fw_related_list {
				if m["@odata.id"] == model2View[mdl].GetURI() {
					add = false
					break
				}
			}
			if add {
				fw_related_list = append(fw_related_list, map[string]interface{}{"@odata.id": model2View[mdl].GetURI()})
			}
		}

		firm_mdl := invview.GetModel("firm")
		firm_mdl.ApplyOption(
			model.UpdateProperty("fw_fqdd_list", fw_fqdd_list),
			model.UpdateProperty("fw_related_list", fw_related_list),
		)

		obsLogger.Info("updated inventory view", "invview", invview, "fw_fqdd_list", fw_fqdd_list, "fw_related_list", fw_related_list)

		// TODO: delete any old copies of this model in the tree
	}

	fn2 := func(mdl *model.Model, updates []model.Update) {
		for _, up := range updates {
			fn(mdl, up.Property, up.NewValue)
		}
	}

	// Set up observers for each swinv model
	obsLogger.Info("Setting up observers", "swinvviews", swinvViews)
	for _, swinvView := range swinvViews {
		// going to assume each view has swinv model at 'swinv'
		mdl := swinvView.GetModel("swinv")
		mdl.AddObserver("swinv", fn2)
		model2View[mdl] = swinvView
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
	sessionModel.AddObserver("viper", func(m *model.Model, updates []model.Update) {
		sessObsLogger.Info("Session variable changed", "model", m, "updates", updates)
		changed := false
		for _, up := range updates {
			if up.Property == "session_timeout" {
				if n, ok := up.NewValue.(int); ok {
					viperMu.Lock()
					cfgMgr.Set("session.timeout", n)
					viperMu.Unlock()
					changed = true
				}
			}
		}
		if changed {
			dumpViperConfig()
		}
	})
	return self
}
