package dell_ec

import (
	"context"
	"strings"
	"sync"

	"io/ioutil"

	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/actionhandler"
	ah "github.com/superchalupa/sailfish/src/actionhandler"
	"github.com/superchalupa/sailfish/src/dell-resources/ar_mapper2"
	"github.com/superchalupa/sailfish/src/dell-resources/attributes"
	"github.com/superchalupa/sailfish/src/dell-resources/certificateservices"
	chasCMCIntegrated "github.com/superchalupa/sailfish/src/dell-resources/chassis/cmc.integrated"
	iom_chassis "github.com/superchalupa/sailfish/src/dell-resources/chassis/iom.slot"
	system_chassis "github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis/power"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis/power/powercontrol"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis/power/powersupply"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis/power/powertrends"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis/subsystemhealth"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis/thermal"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis/thermal/fans"
	sled_chassis "github.com/superchalupa/sailfish/src/dell-resources/chassis/system.modular"
	"github.com/superchalupa/sailfish/src/dell-resources/logservices"
	"github.com/superchalupa/sailfish/src/dell-resources/logservices/faultlist"
	"github.com/superchalupa/sailfish/src/dell-resources/logservices/lcl"
	mgrCMCIntegrated "github.com/superchalupa/sailfish/src/dell-resources/managers/cmc.integrated"
	"github.com/superchalupa/sailfish/src/dell-resources/redundancy"
	"github.com/superchalupa/sailfish/src/dell-resources/registries"
	"github.com/superchalupa/sailfish/src/dell-resources/registries/registry"
	"github.com/superchalupa/sailfish/src/dell-resources/slots"
	"github.com/superchalupa/sailfish/src/dell-resources/slots/slotconfig"
	"github.com/superchalupa/sailfish/src/dell-resources/update_service"
	"github.com/superchalupa/sailfish/src/dell-resources/update_service/firmware_inventory"
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
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	// register all the DM events that are not otherwise pulled in
	_ "github.com/superchalupa/sailfish/src/dell-resources/dm_event"
)

type ocp struct {
	configChangeHandler func()
}

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
}

func (o *ocp) ConfigChangeHandler() { o.configChangeHandler() }

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.Mutex, ch eh.CommandHandler, eb eh.EventBus, d *domain.DomainObjects) *ocp {
	logger = logger.New("module", "ec")
	self := &ocp{}

	updateFns := []func(context.Context, *viper.Viper){}
	swinvViews := []*view.View{}

	// These three all set up a waiter for the root service to appear, so init root service after.
	actionhandler.Setup(ctx, ch, eb)
	evtSvc := eventservice.New(ctx, ch, eb)
	telemetryservice.Setup(ctx, ch, eb)
	event.Setup(ch, eb)
	logSvc := lcl.New(ch, eb)
	faultSvc := faultlist.New(ch, eb)
	slotconfigSvc := slotconfig.New(ch, eb)

	domain.StartInjectService(eb)

	arService, _ := ar_mapper2.StartService(ctx, logger, eb)
	updateFns = append(updateFns, arService.ConfigChangedFn)

	slotSvc := slots.New(arService, ch, eb)

	// the package for this is going to change, but this is what makes the various mappers and view functions available
	testaggregate.RunRegistryFunctions(evtSvc)
	ar_mapper2.RunRegistryFunctions(arService)
	attributes.RunRegistryFunctions(ch, eb)
	expandFormatter := makeExpandListFormatter(d)
	expandOneFormatter := makeExpandOneFormatter(d)

	//
	// Create the (empty) model behind the /redfish/v1 service root. Nothing interesting here
	//
	// No Logger
	// No Model
	// No Controllers
	// View created so we have a place to hold the aggregate UUID and URI
	_, rootView, _ := testaggregate.InstantiateFromCfg(ctx, logger, cfgMgr, "rootview", map[string]interface{}{})
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
	testLogger, testView, _ := testaggregate.InstantiateFromCfg(ctx, logger, cfgMgr, "testview", map[string]interface{}{"rooturi": rootView.GetURI(), "fqdd": "System.Modular.1"})
	testaggregate.AddAggregate(ctx, testView, ch)

	// separately, start goroutine to listen for test events and create sub uris
	testaggregate.StartService(ctx, testLogger, cfgMgr, rootView, ch, eb)

	//*********************************************************************
	//  /redfish/v1/{Managers,Chassis,Systems,Accounts}
	//*********************************************************************
	// Add standard collections: Systems, Chassis, Mangers, Accounts
	//
	stdcollections.AddAggregate(ctx, rootView.GetUUID(), rootView.GetURI(), ch)

	//*********************************************************************
	// /redfish/v1/Sessions
	//*********************************************************************
	_, sessionView, _ := testaggregate.InstantiateFromCfg(ctx, logger, cfgMgr, "sessionview", map[string]interface{}{"rooturi": rootView.GetURI()})
	session.AddAggregate(ctx, sessionView, rootView.GetUUID(), ch, eb)

	//*********************************************************************
	// /redfish/v1/EventService
	// /redfish/v1/TelemetryService
	//*********************************************************************
	evtSvc.StartEventService(ctx, logger, rootView)
	telemetryservice.StartTelemetryService(ctx, logger, rootView)

	//*********************************************************************
	// /redfish/v1/Registries
	//*********************************************************************
	registryLogger, registryView, _ := testaggregate.InstantiateFromCfg(ctx, logger, cfgMgr, "registries", map[string]interface{}{"rooturi": rootView.GetURI()})
	registries.AddAggregate(ctx, registryLogger, registryView, rootView.GetUUID(), ch, eb)

	// TODO: make an adapter for this to move it into redfish.yaml
	// static config controller, initlize values based on yaml config
	staticMapper, _ := static_mapper.New(ctx, registryLogger, registryView.GetModel("default"), "Registries")
	updateFns = append(updateFns, staticMapper.ConfigChangedFn)

	languages := []string{"En"}
	registry_views := []interface{}{}
	for _, registry_map := range []map[string]interface{}{
		{"id": "Messages", "description": "iDRAC Message Registry File locations", "name": "iDRAC Message Registry File", "type": "iDrac.1.5", "location": map[string]string{"Uri": "/redfish/v1/Registries/Messages/EEMIRegistry.v1_5_0.json"}},
		{"id": "BaseMessages", "description": "Base Message Registry File locations", "name": "Base Message Registry File", "type": "Base.1.0", "location": map[string]string{"Uri": "/redfish/v1/Registries/BaseMessages/BaseRegistry.v1_0_0.json", "PublicationUri": "http://www.dmtf.org/sites/default/files/standards/documents/DSP8011_1.0.0a.json"}},
		{"id": "ManagerAttributeRegistry", "description": "Manager Attribute Registry File Locations", "name": "Manager Attribute Registry File", "type": "ManagerAttributeRegistry.1.0", "location": map[string]string{"Uri": "/redfish/v1/Registries/ManagerAttributeRegistry/ManagerAttributeRegistry.v1_0_0.json"}},
	} {

		location := []map[string]string{registry_map["location"].(map[string]string)}
		location[0]["Language"] = "En"
		regModel := model.New(
			model.UpdateProperty("registry_id", registry_map["id"]),
			model.UpdateProperty("registry_description", registry_map["description"]),
			model.UpdateProperty("registry_name", registry_map["name"]),
			model.UpdateProperty("registry_type", registry_map["type"]),
			model.UpdateProperty("languages", languages),
			model.UpdateProperty("languages_count", len(languages)),
			model.UpdateProperty("location", location),
			model.UpdateProperty("location_count", len(location)),
		)

		// static config controller, initlize values based on yaml config
		staticMapper, _ := static_mapper.New(ctx, registryLogger, regModel, "Registries/"+registry_map["id"].(string))
		updateFns = append(updateFns, staticMapper.ConfigChangedFn)

		rv := view.New(
			view.WithURI(rootView.GetURI()+"/Registries/"+registry_map["id"].(string)),
			view.WithModel("default", regModel),
		)
		registry.AddAggregate(ctx, registryLogger, rv, ch, eb)
	}
	registryView.GetModel("default").ApplyOption(model.UpdateProperty("registry_views", &domain.RedfishResourceProperty{Value: registry_views}))

	//HEALTH
	// The following model maps a bunch of health related stuff that can be tracked once at a global level.
	// we can add this model to the views that need to expose it
	globalHealthModel := model.New()
	healthLogger := logger.New("module", "health_rollup")
	awesome_mapper.New(ctx, healthLogger, cfgMgr, globalHealthModel, "global_health", map[string]interface{}{})

	//
	// Loop to create similarly named manager objects and the things attached there.
	//
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
		connectTypesSupported := []interface{}{}

		mgrLogger, mgrCmcVw, _ := testaggregate.InstantiateFromCfg(ctx, logger, cfgMgr, "manager_cmc_integrated",
			map[string]interface{}{
				"rooturi":  rootView.GetURI(),
				"FQDD":     mgrName,                                   // this is used for the AR mapper. case difference is confusing, but need to change mappers
				"fqdd":     "System.Chassis.1#SubSystem.1#" + mgrName, // This is used for the health subsystem
				"fqddlist": []string{mgrName},
			},
		)
		mgrCmcVw.GetModel("default").ApplyOption(
			mgrCMCIntegrated.WithUniqueName(mgrName),
			model.UpdateProperty("unique_name_attr", mgrName+".Attributes"),
			model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),

			model.UpdateProperty("connect_types_supported", connectTypesSupported),
			model.UpdateProperty("connect_types_supported_count", len(connectTypesSupported)),
		)

		mgrCmcVw.ApplyOption(
			view.WithModel("redundancy_health", mgrRedundancyMdl), // health info in default model
			view.WithModel("global_health", globalHealthModel),

			view.WithModel("health", mgrCmcVw.GetModel("default")), // health info in default model
			view.WithModel("swinv", mgrCmcVw.GetModel("default")),  // common name for swinv model, shared in this case
			view.WithModel("default", mgrCmcVw.GetModel("default")),
			view.WithModel("etag", mgrCmcVw.GetModel("default")),

			view.UpdateEtag("etag", []string{}),

			ah.WithAction(ctx, mgrLogger, "manager.reset", "/Actions/Manager.Reset", makePumpHandledAction("ManagerReset", 30, eb), ch, eb),
			ah.WithAction(ctx, mgrLogger, "manager.resettodefaults", "/Actions/Oem/DellManager.ResetToDefaults", makePumpHandledAction("ManagerResetToDefaults", 30, eb), ch, eb),
			ah.WithAction(ctx, mgrLogger, "manager.forcefailover", "/Actions/Manager.ForceFailover", makePumpHandledAction("ManagerForceFailover", 30, eb), ch, eb),
			ah.WithAction(ctx, mgrLogger, "manager.exportsystemconfig", "/Actions/Oem/EID_674_Manager.ExportSystemConfiguration", exportSystemConfiguration, ch, eb),
			ah.WithAction(ctx, mgrLogger, "manager.importsystemconfig", "/Actions/Oem/EID_674_Manager.ImportSystemConfiguration", importSystemConfiguration, ch, eb),
			ah.WithAction(ctx, mgrLogger, "manager.importsystemconfigpreview", "/Actions/Oem/EID_674_Manager.ImportSystemConfigurationPreview", importSystemConfigurationPreview, ch, eb),
			ah.WithAction(ctx, mgrLogger, "certificates.generatecsr", "/Actions/DellCertificateService.GenerateCSR", makePumpHandledAction("GenerateCSR", 30, eb), ch, eb),

			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
			view.WithFormatter("expand", expandFormatter),
			view.WithFormatter("count", countFormatter),
		)

		managers = append(managers, mgrCmcVw)
		swinvViews = append(swinvViews, mgrCmcVw)

		// add the aggregate to the view tree
		mgrCMCIntegrated.AddAggregate(ctx, mgrLogger, mgrCmcVw, ch)
		attributes.AddAggregate(ctx, mgrCmcVw, rootView.GetURI()+"/Managers/"+mgrName+"/Attributes", ch)

		logservices.AddAggregate(ctx, mgrCmcVw, rootView.GetURI()+"/Managers/"+mgrName, ch)
		certificateservices.AddAggregate(ctx, mgrCmcVw, rootView.GetURI()+"/Managers/"+mgrName, ch)

		// Redundancy
		redundancyLogger, redundancyVw, _ := testaggregate.InstantiateFromCfg(ctx, logger, cfgMgr, "chassis_cmc_integrated_redundancy",
			map[string]interface{}{
				"rooturi":  rootView.GetURI(),
				"FQDD":     mgrName,                                   // this is used for the AR mapper. case difference is confusing, but need to change mappers
				"fqdd":     "System.Chassis.1#SubSystem.1#" + mgrName, // This is used for the health subsystem
				"fqddlist": []string{mgrName},
			},
		)

		mgrCmcVw.GetModel("default").ApplyOption(
			model.UpdateProperty("redundancy_uris", []string{redundancyVw.GetURI()}),
		)

		redundancy_set := []string{rootView.GetURI() + "/Managers/CMC.Integrated.1", rootView.GetURI() + "/Managers/CMC.Integrated.2"}

		redundancyVw.GetModel("default").ApplyOption(
			model.UpdateProperty("redundancy_set", redundancy_set),
		)

		redundancy.AddAggregate(ctx, redundancyLogger, redundancyVw, ch)

		//*********************************************************************
		// Create CHASSIS objects for CMC.Integrated.N
		//*********************************************************************
		chasLogger, chasCmcVw, _ := testaggregate.InstantiateFromCfg(ctx, logger, cfgMgr, "chassis_cmc_integrated",
			map[string]interface{}{
				"rooturi":  rootView.GetURI(),
				"FQDD":     mgrName,                            // this is used for the AR mapper. case difference is confusing, but need to change mappers
				"fqdd":     "System.Chassis.1#SubSystem.1#CMC", // This is used for the health subsystem
				"fqddlist": []string{mgrName},
			},
		)

		chasCmcVw.GetModel("default").ApplyOption(
			mgrCMCIntegrated.WithUniqueName(mgrName),
			model.UpdateProperty("unique_name_attr", mgrName+".Attributes"),
			model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),
		)

		chasCmcVw.ApplyOption(
			view.WithModel("etag", chasCmcVw.GetModel("default")),
			view.WithModel("global_health", globalHealthModel),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
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
		sysChasLogger, sysChasVw, _ := testaggregate.InstantiateFromCfg(ctx, logger, cfgMgr, "system_chassis",
			map[string]interface{}{
				"rooturi":  rootView.GetURI(),
				"FQDD":     chasName,
				"fqddlist": []string{chasName},
			},
		)

		managedBy := []string{managers[0].GetURI()}
		sysChasVw.GetModel("default").ApplyOption(
			model.UpdateProperty("unique_name", chasName),
			model.UpdateProperty("unique_name_attr", chasName+".Attributes"),
			model.UpdateProperty("managed_by", managedBy),
			model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),
		)

		sysChasVw.ApplyOption(
			view.WithModel("global_health", globalHealthModel),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
			view.WithFormatter("formatOdataList", FormatOdataList),
			view.WithFormatter("count", countFormatter),
			ah.WithAction(ctx, sysChasLogger, "chassis.reset", "/Actions/Chassis.Reset", makePumpHandledAction("ChassisReset", 30, eb), ch, eb),
			ah.WithAction(ctx, sysChasLogger, "msmconfigbackup", "/Actions/Oem/MSMConfigBackup", msmConfigBackup, ch, eb),
			ah.WithAction(ctx, sysChasLogger, "chassis.msmconfigbackup", "/Actions/Oem/DellChassis.MSMConfigBackup", chassisMSMConfigBackup, ch, eb),
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
		powerLogger, sysChasPwrVw, _ := testaggregate.InstantiateFromCfg(ctx, logger, cfgMgr, "power",
			map[string]interface{}{
				"rooturi": rootView.GetURI(),
				"FQDD":    chasName,
			},
		)

		sysChasPwrVw.GetModel("default").ApplyOption(
			mgrCMCIntegrated.WithUniqueName("Power"),
		)

		sysChasPwrVw.ApplyOption(
			view.WithModel("global_health", globalHealthModel),
			view.WithFormatter("expand", expandFormatter),
			view.WithFormatter("expandone", expandOneFormatter),
			view.WithFormatter("count", countFormatter),
		)
		power.AddAggregate(ctx, powerLogger, sysChasPwrVw, ch)

		psu_uris := []string{}
		for _, psuName := range []string{
			"PSU.Slot.1", "PSU.Slot.2", "PSU.Slot.3",
			"PSU.Slot.4", "PSU.Slot.5", "PSU.Slot.6",
		} {

			psuLogger, sysChasPwrPsuVw, _ := testaggregate.InstantiateFromCfg(ctx, logger, cfgMgr, "psu_slot",
				map[string]interface{}{
					"rooturi":     rootView.GetURI(),
					"FQDD":        psuName, // this is used for the AR mapper. case difference with 'fqdd' is confusing, but need to change mappers
					"ChassisFQDD": chasName,
					"fqdd":        "System.Chassis.1#" + strings.Replace(psuName, "PSU.Slot", "PowerSupply", 1),
					"fqddlist":    []string{psuName},
				},
			)

			sysChasPwrPsuVw.GetModel("default").ApplyOption(
				model.UpdateProperty("unique_name", psuName),
				model.UpdateProperty("unique_name_attr", psuName+".Attributes"),
				model.UpdateProperty("unique_id", psuName),
				model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),
			)

			sysChasPwrPsuVw.ApplyOption(
				view.WithModel("swinv", sysChasPwrPsuVw.GetModel("default")),
				view.WithModel("global_health", globalHealthModel),
				view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
			)
			swinvViews = append(swinvViews, sysChasPwrPsuVw)
			psu_uris = append(psu_uris, sysChasPwrPsuVw.GetURI())
			powersupply.AddAggregate(ctx, psuLogger, sysChasPwrPsuVw, ch)
		}
		sysChasPwrVw.GetModel("default").ApplyOption(model.UpdateProperty("power_supply_uris", psu_uris))

		// ##################
		// # Power Control
		// ##################

		pwrCtrlLogger, sysChasPwrCtrlVw, _ := testaggregate.InstantiateFromCfg(ctx, logger, cfgMgr, "power_control",
			map[string]interface{}{
				"rooturi": rootView.GetURI(),
				"FQDD":    chasName,
			},
		)
		powercontrol.AddAggregate(ctx, pwrCtrlLogger, sysChasPwrCtrlVw, ch)
		sysChasPwrVw.GetModel("default").ApplyOption(model.UpdateProperty("power_control_uris", []string{sysChasPwrCtrlVw.GetURI()}))

		// ##################
		// # Power Trends
		// ##################

		pwrTrendLogger, pwrTrendVw, _ := testaggregate.InstantiateFromCfg(ctx, logger, cfgMgr, "power_trends",
			map[string]interface{}{
				"rooturi": rootView.GetURI(),
				"FQDD":    chasName,
			},
		)
		powertrends.AddTrendsAggregate(ctx, pwrTrendLogger, pwrTrendVw, ch)
		sysChasPwrVw.GetModel("default").ApplyOption(model.UpdateProperty("power_trends_uri", pwrTrendVw.GetURI()))
		pwrTrendVw.ApplyOption(
			view.WithFormatter("expand", expandFormatter),
			view.WithFormatter("expandone", expandOneFormatter),
			view.WithFormatter("count", countFormatter),
		)

		// ##################
		// # Power Histograms
		// ##################

		histogram_uris := []string{}
		for _, trend := range []string{
			"Week", "Day", "Hour",
		} {
			histLogger, histView, _ := testaggregate.InstantiateFromCfg(ctx, logger, cfgMgr, "power_histogram",
				map[string]interface{}{
					"rooturi": rootView.GetURI(),
					"FQDD":    chasName,
					"trend":   trend,
				},
			)
			powertrends.AddHistogramAggregate(ctx, histLogger, histView, ch)
			histogram_uris = append(histogram_uris, histView.GetURI())
		}
		pwrTrendVw.GetModel("default").ApplyOption(model.UpdateProperty("trend_histogram_uris", histogram_uris))

		//*********************************************************************
		// Create Thermal objects for System.Chassis.1
		//*********************************************************************
		thermalLogger, thermalView, _ := testaggregate.InstantiateFromCfg(ctx, logger, cfgMgr, "thermal",
			map[string]interface{}{
				"rooturi": rootView.GetURI(),
				"FQDD":    chasName,
			},
		)

		thermalView.GetModel("default").ApplyOption(
			mgrCMCIntegrated.WithUniqueName("Thermal"),
		)
		// thermal_uris := []string{}
		// redundancy_uris := []string{}

		thermalView.ApplyOption(
			view.WithModel("global_health", globalHealthModel),
			view.WithFormatter("expand", expandFormatter),
			view.WithFormatter("count", countFormatter),
		)
		thermal.AddAggregate(ctx, thermalLogger, thermalView, ch)

		fan_uris := []string{}
		for _, fanName := range []string{
			"Fan.Slot.1", "Fan.Slot.2", "Fan.Slot.3",
			"Fan.Slot.4", "Fan.Slot.5", "Fan.Slot.6",
			"Fan.Slot.7", "Fan.Slot.8", "Fan.Slot.9",
		} {
			fanLogger, fanView, _ := testaggregate.InstantiateFromCfg(ctx, logger, cfgMgr, "fan",
				map[string]interface{}{
					"rooturi":     rootView.GetURI(),
					"ChassisFQDD": chasName,
					"FQDD":        fanName,
					"fqdd":        "System.Chassis.1#" + fanName,
					"fqddlist":    []string{fanName},
				},
			)

			fanView.GetModel("default").ApplyOption(
				model.UpdateProperty("unique_id", fanName),
				model.UpdateProperty("unique_name_attr", fanName+".Attributes"),
				model.UpdateProperty("attributes", map[string]map[string]map[string]interface{}{}),
			)

			fanView.ApplyOption(
				view.WithModel("swinv", fanView.GetModel("default")),
				view.WithModel("global_health", globalHealthModel),
				view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
			)
			fans.AddAggregate(ctx, fanLogger, fanView, ch)
			fan_uris = append(fan_uris, fanView.GetURI())
			swinvViews = append(swinvViews, fanView)
		}
		thermalView.GetModel("default").ApplyOption(model.UpdateProperty("fan_uris", fan_uris))

		//		thermal_views := []interface{}{}
		//		thermalModel.ApplyOption(model.UpdateProperty("thermal_views", &domain.RedfishResourceProperty{Value: thermal_views}))
		//		thermalModel.ApplyOption(model.UpdateProperty("thermal_views_count", len(thermal_views)))
		//
		//		redundancy_views := []interface{}{}
		//		thermalModel.ApplyOption(model.UpdateProperty("redundancy_views", &domain.RedfishResourceProperty{Value: redundancy_views}))
		//		thermalModel.ApplyOption(model.UpdateProperty("redundancy_views_count", len(redundancy_views)))

		//*********************************************************************
		// Create SubSystemHealth for System.Chassis.1
		//*********************************************************************
		subSysHealths := map[string]string{}
		subSysHealthsMap := map[string]interface{}{}

		// TODO: replace this with all healths that are not "absent", use awesome_mapper? or implement perpetual event capture like slots/slotconfig for health events?
		subSysHealths["Battery"] = "OK"

		subSysHealthLogger := sysChasLogger.New("module", "Chassis/System.Chassis/SubSystemHealth")
		subSysHealthModel := model.New()

		armapper := arService.NewMapping(subSysHealthLogger, "Chassis/"+chasName+"/SubSystemHealth", "Chassis/SubSystemHealths", subSysHealthModel, map[string]string{})

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
		managedBy := []string{managers[0].GetURI()}
		iomModel := model.New(
			model.UpdateProperty("unique_name", iomName),
			model.UpdateProperty("unique_name_attr", iomName+".Attributes"),
			model.UpdateProperty("managed_by", managedBy),
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
			view.WithFormatter("formatOdataList", FormatOdataList),
			view.WithFormatter("count", countFormatter),
			ah.WithAction(ctx, iomLogger, "iom.chassis.reset", "/Actions/Chassis.Reset", makePumpHandledAction("IomChassisReset", 30, eb), ch, eb),
			ah.WithAction(ctx, iomLogger, "iom.resetpeakpowerconsumption", "/Actions/Oem/DellChassis.ResetPeakPowerConsumption", makePumpHandledAction("IomResetPeakPowerConsumption", 30, eb), ch, eb),
			ah.WithAction(ctx, iomLogger, "iom.virtualreseat", "/Actions/Oem/DellChassis.VirtualReseat", makePumpHandledAction("IomVirtualReseat", 30, eb), ch, eb),
			evtSvc.PublishResourceUpdatedEventsForModel(ctx, "default"),
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
		sledLogger, sledView, _ := testaggregate.InstantiateFromCfg(ctx, logger, cfgMgr, "sled",
			map[string]interface{}{
				"rooturi":  rootView.GetURI(),
				"FQDD":     sledName,
				"fqdd":     "System.Chassis.1#SubSystem.1#" + sledName,
				"fqddlist": []string{sledName},
			},
		)

		managedBy := []string{managers[0].GetURI()}
		sledView.GetModel("default").ApplyOption(
			model.UpdateProperty("unique_name", sledName),
			model.UpdateProperty("unique_name_attr", sledName+".Attributes"),
			model.UpdateProperty("managed_by", managedBy),
		)

		sledView.ApplyOption(
			view.WithModel("swinv", sledView.GetModel("default")),
			view.WithModel("global_health", globalHealthModel),
			view.WithFormatter("attributeFormatter", attributes.FormatAttributeDump),
			view.WithFormatter("formatOdataList", FormatOdataList),
			view.WithFormatter("count", countFormatter),
			ah.WithAction(ctx, sledLogger, "chassis.peripheralmapping", "/Actions/Oem/DellChassis.PeripheralMapping", makePumpHandledAction("SledPeripheralMapping", 30, eb), ch, eb),
			ah.WithAction(ctx, sledLogger, "sledvirtualreseat", "/Actions/Chassis.VirtualReseat", makePumpHandledAction("SledVirtualReseat", 30, eb), ch, eb),
			ah.WithAction(ctx, sledLogger, "chassis.sledvirtualreseat", "/Actions/Oem/DellChassis.VirtualReseat", makePumpHandledAction("ChassisSledVirtualReseat", 30, eb), ch, eb),
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
			evtSvc.PublishResourceUpdatedEventsForModel(ctx, "default"),
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
				//evtSvc.PublishResourceUpdatedEventsForModel(ctx, "firm"),

				// TODO: oops, can't set up this observer without deadlocking
				// need to figure this one out. This deadlocks taking the lock to add observer
				//				evtSvc.PublishResourceUpdatedEventsForModel(ctx, "swinv"),
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
		sessionView.GetModel("default").ApplyOption(model.UpdateProperty("session_timeout", cfgMgr.GetInt("session.timeout")))

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
	sessionView.GetModel("default").AddObserver("viper", func(m *model.Model, updates []model.Update) {
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
