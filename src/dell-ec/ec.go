package dell_ec

import (
	"context"
	"sync"

	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"

	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	"github.com/superchalupa/sailfish/src/actionhandler"
	"github.com/superchalupa/sailfish/src/eventwaiter"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper"
	"github.com/superchalupa/sailfish/src/ocp/event"
	"github.com/superchalupa/sailfish/src/ocp/eventservice"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/root"
	"github.com/superchalupa/sailfish/src/ocp/session"
	"github.com/superchalupa/sailfish/src/ocp/static_mapper"
	"github.com/superchalupa/sailfish/src/ocp/stdcollections"
	"github.com/superchalupa/sailfish/src/ocp/telemetryservice"
	"github.com/superchalupa/sailfish/src/ocp/test_aggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"

	//"github.com/superchalupa/sailfish/src/dell-resources/ar_mapper"
	"github.com/superchalupa/sailfish/src/dell-resources/ar_mapper2"
	"github.com/superchalupa/sailfish/src/dell-resources/attributes"
	chasCMCIntegrated "github.com/superchalupa/sailfish/src/dell-resources/chassis/cmc.integrated"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/iom.slot"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis/power"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis/power/powercontrol"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis/power/powersupply"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis/power/powertrends"

	"github.com/superchalupa/sailfish/src/dell-resources/certificateservices"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis/subsystemhealth"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis/thermal"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis/thermal/fans"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.modular"
	"github.com/superchalupa/sailfish/src/dell-resources/logservices"
	"github.com/superchalupa/sailfish/src/dell-resources/logservices/faultlist"
	"github.com/superchalupa/sailfish/src/dell-resources/logservices/lcl"
	mgrCMCIntegrated "github.com/superchalupa/sailfish/src/dell-resources/managers/cmc.integrated"
	"github.com/superchalupa/sailfish/src/dell-resources/registries"
	"github.com/superchalupa/sailfish/src/dell-resources/registries/registry"
	"github.com/superchalupa/sailfish/src/dell-resources/slots"
	"github.com/superchalupa/sailfish/src/dell-resources/slots/slotconfig"
	"github.com/superchalupa/sailfish/src/dell-resources/update_service"
	"github.com/superchalupa/sailfish/src/dell-resources/update_service/firmware_inventory"

	// register all the DM events that are not otherwise pulled in
	_ "github.com/superchalupa/sailfish/src/dell-resources/dm_event"

	ah "github.com/superchalupa/sailfish/src/actionhandler"
)

type ocp struct {
	configChangeHandler func()
}

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
}

func (o *ocp) ConfigChangeHandler() { o.configChangeHandler() }

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.Mutex, ch eh.CommandHandler, eb eh.EventBus) *ocp {
	logger = logger.New("module", "ec")
	self := &ocp{}

	updateFns := []func(context.Context, *viper.Viper){}
	swinvViews := []*view.View{}

	// These three all set up a waiter for the root service to appear, so init root service after.
	actionhandler.Setup(ctx, ch, eb)
	eventservice.Setup(ctx, ch, eb)
	telemetryservice.Setup(ctx, ch, eb)
	event.Setup(ch, eb)
	logSvc := lcl.New(ch, eb)
	faultSvc := faultlist.New(ch, eb)
	slotSvc := slot.New(ch, eb)
	slotconfigSvc := slotconfig.New(ch, eb)

	domain.StartInjectService(eb)

	arService, _ := ar_mapper2.StartService(ctx, logger, eb)
	updateFns = append(updateFns, arService.ConfigChangedFn)

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
	awesome_mapper.New(ctx, testLogger, cfgMgr, testModel, "testmodel", map[string]interface{}{"fqdd": "System.Modular.1"})

	ar2mapper := arService.NewMapping(testLogger, "test123", "test/testview", testModel, map[string]string{"FQDD": "happy"})

	testView := view.New(
		view.WithModel("default", testModel),
		view.WithController("ar_mapper", ar2mapper),
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
	armapper := arService.NewMapping(sessionLogger, "SessionService", "SessionService", sessionModel, map[string]string{})

	sessionView := view.New(
		view.WithModel("default", sessionModel),
		view.WithController("ar_mapper", armapper),
		view.WithURI(rootView.GetURI()+"/SessionService"),
		eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
	)
	session.AddAggregate(ctx, sessionView, rootView.GetUUID(), ch, eb)

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
	registries.AddAggregate(ctx, registryLogger, registryView, rootView.GetUUID(), ch, eb)

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
	awesome_mapper.New(ctx, healthLogger, cfgMgr, globalHealthModel, "global_health", map[string]interface{}{})

	//
	// Loop to create similarly named manager objects and the things attached there.
	//
	mgrLogger := logger.New("module", "Managers")
	var managers []*view.View
	mgrRedundancyMdl := model.New()

	related_items := []map[string]string{}

	for _, mgrName := range []string{
		"CMC.Integrated.1",
		"CMC.Integrated.2",
	} {
		//*********************************************************************
		// /redfish/v1/Managers/CMC.Integrated
		//*********************************************************************
		mgrLogger := mgrLogger.New("module", "Managers/"+mgrName, "module", "Managers/CMC.Integrated")
		connectTypesSupported := []interface{}{}
		//managerForChassis := []map[string]string{{"@odata.id": rootView.GetURI()+"/Chassis/System.Chassis.1"}} //sysChasVw.GetURI()
		mdl := model.New(
			mgrCMCIntegrated.WithUniqueName(mgrName),
			model.UpdateProperty("unique_name_attr", mgrName+".Attributes"),
			model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),

			model.UpdateProperty("connect_types_supported", connectTypesSupported),
			model.UpdateProperty("connect_types_supported_count", len(connectTypesSupported)),

			// manually add health properties until we get a mapper to automatically manage these
		)

		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		fwmapper := arService.NewMapping(mgrLogger.New("module", "firmware/inventory"), "fwmapper_Managers/"+mgrName, "firmware/inventory", mdl, map[string]string{"FQDD": mgrName})

		// need to have a separate model to hold fpga ver
		//fpgamapper, _ := arService.NewMapping(mgrLogger, "fpgamapper_Managers/"+mgrName, "fpga_inventory", mdl, map[string]string{"FQDD": mgrName})
		armapper := arService.NewMapping(mgrLogger, "Managers/"+mgrName, "Managers/CMC.Integrated", mdl, map[string]string{"FQDD": mgrName})

		// This controller will populate 'attributes' property with AR entries matching this FQDD ('mgrName')
		ardumper, _ := attributes.NewController(ctx, mdl, []string{mgrName}, ch, eb)

		awesome_mapper.New(ctx, mgrLogger, cfgMgr, mdl, "health", map[string]interface{}{"fqdd": "System.Chassis.1#SubSystem.1#" + mgrName})

		mgrCmcVw := view.New(
			view.WithURI(rootView.GetURI()+"/Managers/"+mgrName),
			view.WithModel("redundancy_health", mgrRedundancyMdl), // health info in default model
			view.WithModel("health", mdl),                         // health info in default model
			view.WithModel("global_health", globalHealthModel),
			view.WithModel("swinv", mdl), // common name for swinv model, shared in this case
			view.WithModel("default", mdl),
			view.WithModel("etag", mdl),
			view.WithController("ar_mapper", armapper),
			view.WithController("ar_dump", ardumper),
			view.WithController("fw_mapper", fwmapper),
			view.UpdateEtag("etag", []string{}),

			ah.WithAction(ctx, mgrLogger, "manager.reset", "/Actions/Manager.Reset", makePumpHandledAction("ManagerReset", 30, eb), ch, eb),
			ah.WithAction(ctx, mgrLogger, "manager.resettodefaults", "/Actions/Oem/DellManager.ResetToDefaults", makePumpHandledAction("ManagerResetToDefaults", 30, eb), ch, eb),
			ah.WithAction(ctx, mgrLogger, "manager.forcefailover", "/Actions/Manager.ForceFailover", makePumpHandledAction("ManagerForceFailover", 30, eb), ch, eb),
			ah.WithAction(ctx, mgrLogger, "manager.exportsystemconfig", "/Actions/Oem/EID_674_Manager.ExportSystemConfiguration", exportSystemConfiguration, ch, eb),
			ah.WithAction(ctx, mgrLogger, "manager.importsystemconfig", "/Actions/Oem/EID_674_Manager.ImportSystemConfiguration", importSystemConfiguration, ch, eb),
			ah.WithAction(ctx, mgrLogger, "manager.importsystemconfigpreview", "/Actions/Oem/EID_674_Manager.ImportSystemConfigurationPreview", importSystemConfigurationPreview, ch, eb),
			ah.WithAction(ctx, mgrLogger, "certificates.generatecsr", "/Actions/DellCertificateService.GenerateCSR", makePumpHandledAction("GenerateCSR", 30, eb), ch, eb),

			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
			eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
		)

		managers = append(managers, mgrCmcVw)
		swinvViews = append(swinvViews, mgrCmcVw)

		// add the aggregate to the view tree
		mgrCMCIntegrated.AddAggregate(ctx, mgrLogger, mgrCmcVw, ch)
		attributes.AddAggregate(ctx, mgrCmcVw, rootView.GetURI()+"/Managers/"+mgrName+"/Attributes", ch)

		logservices.AddAggregate(ctx, mgrCmcVw, rootView.GetURI()+"/Managers/"+mgrName, ch)
		certificateservices.AddAggregate(ctx, mgrCmcVw, rootView.GetURI()+"/Managers/"+mgrName, ch)


		// Redundancy
		redundancy_set := []map[string]string{{"@odata.id": rootView.GetURI()+"/Managers/CMC.Integrated.1"}, {"@odata.id": rootView.GetURI()+"/Managers/CMC.Integrated.2"}}
		redundancy_views := []interface{}{}
		redundancyLogger := logger.New("module", "Managers/"+mgrName+"/Redundancy")
		redundancyModel := model.New(
			model.UpdateProperty("unique_id", rootView.GetURI()+"/Managers/"+mgrName+"#Redundancy"),
			model.UpdateProperty("redundancy_set", redundancy_set),
			model.UpdateProperty("redundancy_set_count", len(redundancy_set)),
		)
		armapper = arService.NewMapping(redundancyLogger, "Managers/"+mgrName+"/Redundancy", "Managers/CMC.Integrated", redundancyModel, map[string]string{"FQDD": mgrName})

		awesome_mapper.New(ctx, redundancyLogger, cfgMgr, redundancyModel, "health", map[string]interface{}{"fqdd": "System.Chassis.1#SubSystem.1#" + mgrName})

		redundancyVw := view.New(
			view.WithURI(rootView.GetURI()+"/Managers/"+mgrName+"/Redundancy"),
			view.WithModel("default", redundancyModel),
			view.WithController("ar_mapper", armapper),
			eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
		)
		redundancy := redundancy.AddAggregate(ctx, redundancyLogger, redundancyVw, ch)
		p := &domain.RedfishResourceProperty{}
		p.Parse(redundancy)
		redundancy_views = append(redundancy_views, p)

		mdl.ApplyOption(model.UpdateProperty("redundancy_views", &domain.RedfishResourceProperty{Value: redundancy_views}))
		mdl.ApplyOption(model.UpdateProperty("redundancy_views_count", len(redundancy_views)))




		//*********************************************************************
		// Create CHASSIS objects for CMC.Integrated.N
		//*********************************************************************
		chasLogger := logger.New("module", "Chassis/"+mgrName, "module", "Chassis/CMC.Integrated")
		chasModel, _ := mgrCMCIntegrated.New(
			mgrCMCIntegrated.WithUniqueName(mgrName),
			model.UpdateProperty("unique_name_attr", mgrName+".Attributes"),
			model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),
		)
		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish... re-use the same mappings from Managers
		armapper = arService.NewMapping(chasLogger, "Chassis/"+mgrName, "Managers/CMC.Integrated", chasModel, map[string]string{"FQDD": mgrName})

		// This controller will populate 'attributes' property with AR entries matching this FQDD ('mgrName')
		ardumper, _ = attributes.NewController(ctx, chasModel, []string{mgrName}, ch, eb)

		awesome_mapper.New(ctx, chasLogger, cfgMgr, chasModel, "health", map[string]interface{}{"fqdd": "System.Chassis.1#SubSystem.1#CMC"})

		chasCmcVw := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+mgrName),
			view.WithModel("default", chasModel),
			view.WithModel("etag", mdl),
			view.WithModel("global_health", globalHealthModel),
			view.WithController("ar_mapper", armapper),
			view.WithController("ar_dump", ardumper),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
			eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
			view.UpdateEtag("etag", []string{}),
		)

		// add the aggregate to the view tree
		chasCMCIntegrated.AddAggregate(ctx, chasLogger, chasCmcVw, ch)
		attributes.AddAggregate(ctx, chasCmcVw, rootView.GetURI()+"/Chassis/"+mgrName+"/Attributes", ch)

		related_items = append(related_items, map[string]string{"@odata.id": chasCmcVw.GetURI()})

	}

	// start log service here: it attaches to cmc.integrated.1
	logSvc.StartService(ctx, logger, managers[0])
	faultSvc.StartService(ctx, logger, managers[0])

	pwrCtrlModel := model.New()

	chasLogger := logger.New("module", "Chassis")
	{
		// ************************************************************************
		// CHASSIS System.Chassis.1
		// ************************************************************************
		chasName := "System.Chassis.1"
		sysChasLogger := chasLogger.New("module", "Chassis/"+chasName, "module", "Chassis/System.Chassis")
		managedBy := []map[string]string{{"@odata.id": managers[0].GetURI()}}
		chasModel := model.New(
			model.UpdateProperty("unique_name", chasName),
			model.UpdateProperty("unique_name_attr", chasName+".Attributes"),
			model.UpdateProperty("managed_by", managedBy),
			model.UpdateProperty("managed_by_count", len(managedBy)),
			model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),
		)
		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		armapper = arService.NewMapping(sysChasLogger, "Chassis/"+chasName, "Chassis/System.Chassis", chasModel, map[string]string{"FQDD": chasName})

		// This controller will populate 'attributes' property with AR entries matching this FQDD ('chasName')
		ardumper, _ := attributes.NewController(ctx, chasModel, []string{chasName}, ch, eb)

		sysChasVw := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+chasName),
			view.WithModel("default", chasModel),
			view.WithModel("global_health", globalHealthModel),
			view.WithController("ar_mapper", armapper),
			view.WithController("ar_dump", ardumper),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
			ah.WithAction(ctx, sysChasLogger, "chassis.reset", "/Actions/Chassis.Reset", makePumpHandledAction("ChassisReset", 30, eb), ch, eb),
			ah.WithAction(ctx, sysChasLogger, "msmconfigbackup", "/Actions/Oem/MSMConfigBackup", msmConfigBackup, ch, eb),
			ah.WithAction(ctx, sysChasLogger, "chassis.msmconfigbackup", "/Actions/Oem/DellChassis.MSMConfigBackup", chassisMSMConfigBackup, ch, eb),
			eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
		)

		// Create the .../Attributes URI. Attributes are stored in the attributes property of the chasModel
		system_chassis.AddAggregate(ctx, sysChasLogger, sysChasVw, ch, eb)
		related_items = append(related_items, map[string]string{"@odata.id": sysChasVw.GetURI()})
		attributes.AddAggregate(ctx, sysChasVw, rootView.GetURI()+"/Chassis/"+chasName+"/Attributes", ch)

		// CMC.INTEGRATED.1 INTERLUDE
		managerForChassis := []map[string]string{{"@odata.id": sysChasVw.GetURI()}}
		mgr_mdl := managers[0].GetModel("default")
		mgr_mdl.UpdateProperty("manager_for_chassis", managerForChassis)
		mgr_mdl.UpdateProperty("manager_for_chassis_count", len(managerForChassis))

		//*********************************************************************
		// Create Power objects for System.Chassis.1
		//*********************************************************************
		powerLogger := sysChasLogger.New("module", "Chassis/System.Chassis/Power")

		powerModel := model.New(
			mgrCMCIntegrated.WithUniqueName("Power"),
			model.UpdateProperty("power_supply_views", []interface{}{}),
			model.UpdateProperty("power_control_views", []interface{}{}),
			model.UpdateProperty("power_trend_views", []interface{}{}),
		)
		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		armapper = arService.NewMapping(powerLogger, "Chassis/"+chasName+"/Power", "Chassis/System.Chassis/Power", powerModel, map[string]string{"FQDD": chasName})

		sysChasPwrVw := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+chasName+"/Power"),
			view.WithModel("default", powerModel),
			view.WithModel("global_health", globalHealthModel),
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
				model.UpdateProperty("unique_name_attr", psuName+".Attributes"),
				model.UpdateProperty("unique_id", psuName),
				model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),
			)
			fwmapper := arService.NewMapping(psuLogger.New("module", "firmware/inventory"), "firmware_Chassis/"+chasName+"/Power/PowerSupplies/"+psuName, "firmware/inventory", psuModel, map[string]string{"FQDD": psuName})

			// the controller is what updates the model when ar entries change,
			// also handles patch from redfish
			armapper := arService.NewMapping(psuLogger, "Chassis/"+chasName+"/Power/PowerSupplies/"+psuName, "PSU/PSU.Slot", psuModel, map[string]string{"FQDD": psuName})

			// This controller will populate 'attributes' property with AR entries matching this FQDD ('psuName')
			ardumper, _ := attributes.NewController(ctx, psuModel, []string{psuName}, ch, eb)

			sysChasPwrPsuVw := view.New(
				view.WithURI(rootView.GetURI()+"/Chassis/"+chasName+"/Power/PowerSupplies/"+psuName),
				view.WithModel("default", psuModel),
				view.WithModel("swinv", psuModel),
				view.WithModel("global_health", globalHealthModel),
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
		powerModel.ApplyOption(model.UpdateProperty("power_supply_views_count", len(psu_views)))

		pwrCtrl_views := []interface{}{}
		pwrCtrlLogger := sysChasLogger.New("module", "Chassis/System.Chassis/Power/PowerControl")

		armapper = arService.NewMapping(pwrCtrlLogger, "Chassis/"+chasName+"/Power/PowerControl", "Chassis/System.Chassis/Power", pwrCtrlModel, map[string]string{"FQDD": chasName})

		// Power consumption in kwh TODO
		awesome_mapper.New(ctx, pwrCtrlLogger, cfgMgr, pwrCtrlModel, "power", map[string]interface{}{})

		sysChasPwrCtrlVw := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+chasName+"/Power/PowerControl"),
			view.WithModel("default", pwrCtrlModel),
			view.WithController("ar_mapper", armapper),
			eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
		)
		pwrCtrl := powercontrol.AddAggregate(ctx, pwrCtrlLogger, sysChasPwrCtrlVw, ch)

		p := &domain.RedfishResourceProperty{}
		p.Parse(pwrCtrl)
		pwrCtrl_views = append(pwrCtrl_views, p)

		powerModel.ApplyOption(model.UpdateProperty("power_control_views", &domain.RedfishResourceProperty{Value: pwrCtrl_views}))
		powerModel.ApplyOption(model.UpdateProperty("power_control_views_count", len(pwrCtrl_views)))

		trend_views := []interface{}{}

		pwrTrendLogger := sysChasLogger.New("module", "Chassis/System.Chassis/Power/PowerTrends")
		pwrTrendModel := model.New(
			model.UpdateProperty("histograms", []interface{}{}),
		)

		armapper = arService.NewMapping(pwrTrendLogger, "Chassis/"+chasName+"/Power/PowerTrends", "Chassis/System.Chassis/Power", pwrTrendModel, map[string]string{"FQDD": chasName})

		pwrTrendVw := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+chasName+"/Power/PowerTrends-1"),
			view.WithModel("default", pwrTrendModel),
			view.WithController("ar_mapper", armapper),
			eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
		)
		pwrTrend := powertrends.AddAggregate(ctx, pwrTrendLogger, pwrTrendVw, false, ch)
		p = &domain.RedfishResourceProperty{}
		p.Parse(pwrTrend)
		trend_views = append(trend_views, p)

		histogram_views := []interface{}{}
		for _, trend := range []string{
			"LastWeek", "LastDay", "LastHour",
		} {
			trendModel := model.New()
			armapper := arService.NewMapping(pwrTrendLogger, "Chassis/"+chasName+"/Power/PowerTrends-1/"+trend, "Chassis/System.Chassis/Power", trendModel, map[string]string{"FQDD": chasName})
			trendView := view.New(
				view.WithURI(rootView.GetURI()+"/Chassis/"+chasName+"/Power/PowerTrends-1/"+trend),
				view.WithModel("default", trendModel),
				view.WithController("ar_mapper", armapper),
				eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
			)
			trend := powertrends.AddAggregate(ctx, pwrTrendLogger, trendView, true, ch)
			p := &domain.RedfishResourceProperty{}
			p.Parse(trend)
			histogram_views = append(histogram_views, p)
		}

		pwrTrendModel.ApplyOption(model.UpdateProperty("histograms", &domain.RedfishResourceProperty{Value: histogram_views}))
		pwrTrendModel.ApplyOption(model.UpdateProperty("histograms_count", len(histogram_views)))

		powerModel.ApplyOption(model.UpdateProperty("power_trend_views", &domain.RedfishResourceProperty{Value: trend_views}))
		powerModel.ApplyOption(model.UpdateProperty("power_trend_count", len(trend_views)))

		//*********************************************************************
		// Create Thermal objects for System.Chassis.1
		//*********************************************************************
		thermalLogger := sysChasLogger.New("module", "Chassis/System.Chassis/Thermal")

		thermalModel := model.New(
			mgrCMCIntegrated.WithUniqueName("Thermal"),
			model.UpdateProperty("fan_views", []interface{}{}),
			model.UpdateProperty("thermal_views", []interface{}{}),
			model.UpdateProperty("redundancy_views", []interface{}{}),
		)
		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		armapper := arService.NewMapping(thermalLogger, "Chassis/"+chasName+"/Thermal", "Chassis/System.Chassis/Thermal", thermalModel, map[string]string{"FQDD": chasName})

		thermalView := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+chasName+"/Thermal"),
			view.WithModel("default", thermalModel),
			view.WithModel("global_health", globalHealthModel),
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
				model.UpdateProperty("unique_name_attr", fanName+".Attributes"),
				model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),
			)
			fwmapper := arService.NewMapping(fanLogger.New("module", "firmware/inventory"), "firmware_Chassis/"+chasName+"/Thermal/Fan/"+fanName, "firmware/inventory", fanModel, map[string]string{"FQDD": fanName})
			// the controller is what updates the model when ar entries change,
			// also handles patch from redfish
			armapper := arService.NewMapping(fanLogger, "Chassis/"+chasName+"/Thermal/Fan/"+fanName, "Fans/Fan.Slot", fanModel, map[string]string{"FQDD": fanName})

			awesome_mapper.New(ctx, fanLogger, cfgMgr, fanModel, "fan", map[string]interface{}{"fqdd": "System.Chassis.1#" + fanName})

			// This controller will populate 'attributes' property with AR entries matching this FQDD ('fanName')
			ardumper, _ := attributes.NewController(ctx, fanModel, []string{fanName}, ch, eb)

			v := view.New(
				view.WithURI(rootView.GetURI()+"/Chassis/"+chasName+"/Sensors/Fans/"+fanName),
				view.WithModel("default", fanModel),
				view.WithModel("swinv", fanModel),
				view.WithModel("global_health", globalHealthModel),
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
		thermalModel.ApplyOption(model.UpdateProperty("fan_views_count", len(fan_views)))

		thermal_views := []interface{}{}
		thermalModel.ApplyOption(model.UpdateProperty("thermal_views", &domain.RedfishResourceProperty{Value: thermal_views}))
		thermalModel.ApplyOption(model.UpdateProperty("thermal_views_count", len(thermal_views)))

		redundancy_views := []interface{}{}
		thermalModel.ApplyOption(model.UpdateProperty("redundancy_views", &domain.RedfishResourceProperty{Value: redundancy_views}))
		thermalModel.ApplyOption(model.UpdateProperty("redundancy_views_count", len(redundancy_views)))

		//*********************************************************************
		// Create SubSystemHealth for System.Chassis.1
		//*********************************************************************
		subSysHealths := map[string]string{}
		subSysHealthsMap := map[string]interface{}{}

		// TODO: replace this with all healths that are not "absent", use awesome_mapper? or implement perpetual event capture like slots/slotconfig for health events?
		subSysHealths["Battery"] = "OK"

		subSysHealthLogger := sysChasLogger.New("module", "Chassis/System.Chassis/SubSystemHealth")
		subSysHealthModel := model.New()

		armapper = arService.NewMapping(subSysHealthLogger, "Chassis/"+chasName+"/SubSystemHealth", "Chassis/SubSystemHealths", subSysHealthModel, map[string]string{})

		subSysHealthView := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+chasName+"/SubSystemHealth"),
			view.WithModel("default", subSysHealthModel),
			view.WithController("ar_mapper", armapper),
		)

		subsystemhealth.AddAggregate(ctx, subSysHealthLogger, subSysHealthView, ch, eb)

		for key, value := range subSysHealths {
			subSysHealthsMap[key] = map[string]interface{}{
				"Status": map[string]string{
					"HealthRollup": value,
				},
			}
		}
		subSysHealthModel.ApplyOption(model.UpdateProperty("subsystems", &domain.RedfishResourceProperty{Value: subSysHealthsMap}))

		/*  Slots */
		//slotSvc.StartService(ctx, logger, sysChasVw, cfgMgr, arService)
		slotSvc.StartService(ctx, logger, sysChasVw, cfgMgr, updateFns, ch, eb)

		/* Slot config */
		//slotconfigSvc.StartService(ctx, logger, sysChasVw, cfgMgr, arService)
		slotconfigSvc.StartService(ctx, logger, sysChasVw, cfgMgr, updateFns, ch, eb)

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
		managedBy := []map[string]string{{"@odata.id": managers[0].GetURI()}}
		iomModel := model.New(
			model.UpdateProperty("unique_name", iomName),
			model.UpdateProperty("unique_name_attr", iomName+".Attributes"),
			model.UpdateProperty("managed_by", managedBy),
			model.UpdateProperty("managed_by_count", len(managedBy)),
		)
		fwmapper := arService.NewMapping(iomLogger.New("module", "firmware/inventory"), "firmware_Chassis/"+iomName, "firmware/inventory", iomModel, map[string]string{"FQDD": iomName})
		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		armapper := arService.NewMapping(iomLogger, "Chassis/"+iomName, "Chassis/IOM.Slot", iomModel, map[string]string{"FQDD": iomName})

		// This controller will populate 'attributes' property with AR entries matching this FQDD ('iomName')
		ardumper, _ := attributes.NewController(ctx, iomModel, []string{iomName}, ch, eb)

		//HEALTH
		awesome_mapper.New(ctx, iomLogger, cfgMgr, iomModel, "health", map[string]interface{}{"fqdd": "System.Chassis.1#SubSystem.1#" + iomName})

		//INST POWER CONSUMPTION
		awesome_mapper.New(ctx, iomLogger, cfgMgr, iomModel, "iom", map[string]interface{}{"fqdd": iomName})

		iomView := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+iomName),
			view.WithModel("default", iomModel),
			view.WithModel("swinv", iomModel),
			view.WithModel("global_health", globalHealthModel),
			view.WithController("ar_mapper", armapper),
			view.WithController("fw_mapper", fwmapper),
			view.WithController("ar_dumper", ardumper),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
			ah.WithAction(ctx, iomLogger, "iom.chassis.reset", "/Actions/Chassis.Reset", makePumpHandledAction("IomChassisReset", 30, eb), ch, eb),
			ah.WithAction(ctx, iomLogger, "iom.resetpeakpowerconsumption", "/Actions/Oem/DellChassis.ResetPeakPowerConsumption", makePumpHandledAction("IomResetPeakPowerConsumption", 30, eb), ch, eb),
			ah.WithAction(ctx, iomLogger, "iom.virtualreseat", "/Actions/Oem/DellChassis.VirtualReseat", makePumpHandledAction("IomVirtualReseat", 30, eb), ch, eb),
			eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
		)
		swinvViews = append(swinvViews, iomView)
		related_items = append(related_items, map[string]string{"@odata.id": iomView.GetURI()})
		iom_chassis.AddAggregate(ctx, iomLogger, iomView, ch, eb)
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
		sledLogger := chasLogger.New("module", "Chassis/"+sledName, "module", "Chassis/System.Modular")
		managedBy := []map[string]string{{"@odata.id": managers[0].GetURI()}}
		sledModel := model.New(
			model.UpdateProperty("unique_name", sledName),
			model.UpdateProperty("unique_name_attr", sledName+".Attributes"),
			model.UpdateProperty("managed_by", managedBy),
			model.UpdateProperty("managed_by_count", len(managedBy)),
		)
		fwmapper := arService.NewMapping(sledLogger.New("module", "firmware/inventory"), "firmware_Chassis/"+sledName, "firmware/inventory", sledModel, map[string]string{"FQDD": sledName})

		armapper := arService.NewMapping(sledLogger, "Chassis/"+sledName, "Chassis/System.Modular", sledModel, map[string]string{"FQDD": sledName})

		// This controller will populate 'attributes' property with AR entries matching this FQDD ('sledName')
		ardumper, _ := attributes.NewController(ctx, sledModel, []string{sledName}, ch, eb)

		//HEALTH
		awesome_mapper.New(ctx, sledLogger, cfgMgr, sledModel, "health", map[string]interface{}{"fqdd": "System.Chassis.1#SubSystem.1#" + sledName})

		sledView := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+sledName),
			view.WithModel("default", sledModel),
			view.WithModel("swinv", sledModel),
			view.WithModel("global_health", globalHealthModel),
			view.WithController("ar_mapper", armapper),
			view.WithController("fw_mapper", fwmapper),
			view.WithController("ar_dumper", ardumper),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
			ah.WithAction(ctx, sledLogger, "chassis.peripheralmapping", "/Actions/Oem/DellChassis.PeripheralMapping", makePumpHandledAction("SledPeripheralMapping", 30, eb), ch, eb),
			ah.WithAction(ctx, sledLogger, "sledvirtualreseat", "/Actions/Chassis.VirtualReseat", makePumpHandledAction("SledVirtualReseat", 30, eb), ch, eb),
			ah.WithAction(ctx, sledLogger, "chassis.sledvirtualreseat", "/Actions/Oem/DellChassis.VirtualReseat", makePumpHandledAction("ChassisSledVirtualReseat", 30, eb), ch, eb),
			eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
		)
		sled_chassis.AddAggregate(ctx, sledLogger, sledView, ch, eb)
		related_items = append(related_items, map[string]string{"@odata.id": sledView.GetURI()})
		attributes.AddAggregate(ctx, sledView, rootView.GetURI()+"/Chassis/"+sledName+"/Attributes", ch)
	}

	{
		updsvcLogger := logger.New("module", "UpdateService")
		mdl := model.New()

		// the controller is what updates the model when ar entries change,
		// also handles patch from redfish
		armapper := arService.NewMapping(updsvcLogger, "Chassis", "update_service", mdl, map[string]string{})

		updSvcVw := view.New(
			view.WithURI(rootView.GetURI()+"/UpdateService"),
			view.WithModel("default", mdl),
			view.WithController("ar_mapper", armapper),
			ah.WithAction(ctx, updsvcLogger, "update.reset", "/Actions/Oem/DellUpdateService.Reset", updateReset, ch, eb),
			ah.WithAction(ctx, updsvcLogger, "update.eid674.reset", "/Actions/Oem/EID_674_UpdateService.Reset", updateEID674Reset, ch, eb),
			ah.WithAction(ctx, updsvcLogger, "update.syncup", "/Actions/Oem/DellUpdateService.Syncup", makePumpHandledAction("UpdateSyncup", 30, eb), ch, eb),
			ah.WithAction(ctx, updsvcLogger, "update.eid674.syncup", "/Actions/Oem/EID_674_UpdateService.Syncup", makePumpHandledAction("UpdateSyncup", 30, eb), ch, eb),
			eventservice.PublishResourceUpdatedEventsForModel(ctx, "default", eb),
		)

		// add the aggregate to the view tree
		update_service.AddAggregate(ctx, rootView, updSvcVw, ch)
		update_service.EnhanceAggregate(ctx, updSvcVw, rootView, ch)
	}

	pwrCtrlModel.ApplyOption(model.UpdateProperty("related_item", related_items))
	pwrCtrlModel.ApplyOption(model.UpdateProperty("related_item_count", len(related_items)))

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
	// IMPORTANT: this function is called with the model lock held! You can't
	// call any functions on the model that take the model lock, or it will
	// deadlock.
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
				// TODO: oops, no clue why, but this deadlocks for some insane reason
				//eventservice.PublishResourceUpdatedEventsForModel(ctx, "firm", eb),

				// TODO: oops, can't set up this observer without deadlocking
				// need to figure this one out. This deadlocks taking the lock to add observer
				//				eventservice.PublishResourceUpdatedEventsForModel(ctx, "swinv", eb),
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
