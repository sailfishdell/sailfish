package dell_ec

import (
	"context"
	"strings"
	"sync"

	"github.com/spf13/viper"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/actionhandler"
	ah "github.com/superchalupa/sailfish/src/actionhandler"
	"github.com/superchalupa/sailfish/src/dell-resources/ar_mapper2"
	"github.com/superchalupa/sailfish/src/dell-resources/attributes"
	"github.com/superchalupa/sailfish/src/dell-resources/certificateservices"
	chasCMCIntegrated "github.com/superchalupa/sailfish/src/dell-resources/chassis/cmc.integrated"
	iom_chassis "github.com/superchalupa/sailfish/src/dell-resources/chassis/iom.slot"
	iom_config "github.com/superchalupa/sailfish/src/dell-resources/chassis/iom.slot/iomconfig"
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
	"github.com/superchalupa/sailfish/src/dell-resources/slots"
	"github.com/superchalupa/sailfish/src/dell-resources/slots/slotconfig"
	"github.com/superchalupa/sailfish/src/dell-resources/update_service"
	"github.com/superchalupa/sailfish/src/dell-resources/update_service/firmware_inventory"
	"github.com/superchalupa/sailfish/src/eventwaiter"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/event"
	"github.com/superchalupa/sailfish/src/ocp/eventservice"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/session"
	"github.com/superchalupa/sailfish/src/ocp/stdcollections"
	"github.com/superchalupa/sailfish/src/ocp/telemetryservice"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
	"github.com/superchalupa/sailfish/src/stdmeta"
	"github.com/superchalupa/sailfish/src/uploadhandler"

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
	uploadhandler.Setup(ctx, ch, eb)
	telemetryservice.Setup(ctx, ch, eb)
	event.Setup(ch, eb)
	logSvc := lcl.New(ch, eb)
	faultSvc := faultlist.New(ch, eb)
	domain.StartInjectService(eb)
	arService, _ := ar_mapper2.StartService(ctx, logger, cfgMgr, eb)
	actionSvc := ah.StartService(ctx, logger, ch, eb)
	uploadSvc := uploadhandler.StartService(ctx, logger, ch, eb)
	am2Svc, _ := awesome_mapper2.StartService(ctx, logger, eb)
	pumpSvc := NewPumpActionSvc(ctx, logger, eb)

	subSystemSvc := subsystemhealth.New(ch, eb)
	slotconfigSvc := slotconfig.New(ch, eb)

	// the package for this is going to change, but this is what makes the various mappers and view functions available
	instantiateSvc := testaggregate.New(logger, ch)
	evtSvc := eventservice.New(ctx, cfgMgr, instantiateSvc, ch, eb)
	testaggregate.RegisterWithURI(instantiateSvc)
	testaggregate.RegisterPublishEvents(instantiateSvc, evtSvc)
	testaggregate.RegisterAggregate(instantiateSvc)
	testaggregate.RegisterAM2(instantiateSvc, am2Svc)
	ar_mapper2.RegisterARMapper(instantiateSvc, arService)
	attributes.RegisterARMapper(instantiateSvc, ch, eb)
	stdmeta.RegisterFormatters(instantiateSvc, d)
	registries.RegisterAggregate(instantiateSvc)
	stdcollections.RegisterAggregate(instantiateSvc)
	session.RegisterAggregate(instantiateSvc)
	eventservice.RegisterAggregate(instantiateSvc)
	slots.RegisterAggregate(instantiateSvc)
	logservices.RegisterAggregate(instantiateSvc)

	// common parameters to instantiate that are used almost everywhere
	baseParams := map[string]interface{}{}
	baseParams["rooturi"] = "/redfish/v1"
	modParams := func(newParams map[string]interface{}) map[string]interface{} {
		ret := map[string]interface{}{}
		for k, v := range baseParams {
			ret[k] = v
		}
		for k, v := range newParams {
			ret[k] = v
		}
		return ret
	}

	//HEALTH
	// The following model maps a bunch of health related stuff that can be tracked once at a global level.
	// we can add this model to the views that need to expose it
	globalHealthModel := model.New()
	healthLogger := logger.New("module", "health_rollup")
	am2Svc.NewMapping(ctx, healthLogger, cfgMgr, globalHealthModel, "global_health", "global_health", map[string]interface{}{})

	//*********************************************************************
	//  /redfish/v1
	//*********************************************************************
	_, rootView, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "rootview", baseParams)
	baseParams["rootid"] = rootView.GetUUID()

	//*********************************************************************
	//  /redfish/v1/testview - a proof of concept test view and example
	//*********************************************************************
	instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "testview", baseParams)

	//*********************************************************************
	//  /redfish/v1/{Managers,Chassis,Systems,Accounts}
	//*********************************************************************
	_, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "chassis", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/Chassis"}))
	_, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "systems", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/Systems"}))
	_, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "managers", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/Managers"}))
	_, accountSvcVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "accountservice", modParams(map[string]interface{}{}))
	baseParams["actsvc_uri"] = accountSvcVw.GetURI()
	baseParams["actsvc_id"] = accountSvcVw.GetUUID()
	_, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "roles", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/AccountService/Roles"}))
	_, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "accounts", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/AccountService/Accounts"}))

	//*********************************************************************
	//  Standard redfish roles
	//*********************************************************************
	stdcollections.AddStandardRoles(ctx, rootView.GetUUID(), rootView.GetURI(), ch)

	//*********************************************************************
	// /redfish/v1/Sessions
	//*********************************************************************
	_, sessionSvcVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "sessionservice", baseParams)
	baseParams["sessionsvc_id"] = sessionSvcVw.GetUUID()
	baseParams["sessionsvc_uri"] = sessionSvcVw.GetURI()
	session.SetupSessionService(ctx, instantiateSvc, sessionSvcVw, cfgMgr, ch, eb, baseParams)
	instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "sessioncollection", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/SessionService/Sessions"}))

	//*********************************************************************
	// /redfish/v1/EventService
	// /redfish/v1/TelemetryService
	//*********************************************************************
	evtSvc.StartEventService(ctx, logger, instantiateSvc, baseParams)
	telemetryservice.StartTelemetryService(ctx, logger, rootView)

	//*********************************************************************
	// /redfish/v1/Registries
	//*********************************************************************
	instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "registries", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/Registries"}))

	for regName, location := range map[string]interface{}{
		"idrac_registry":    []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/Messages/EEMIRegistry.v1_5_0.json"}},
		"base_registry":     []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/BaseMessages/BaseRegistry.v1_0_0.json", "PublicationUri": "http://www.dmtf.org/sites/default/files/standards/documents/DSP8011_1.0.0a.json"}},
		"mgr_attr_registry": []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/ManagerAttributeRegistry/ManagerAttributeRegistry.v1_0_0.json"}},
	} {
		instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, regName, modParams(map[string]interface{}{"location": location}))
	}

	// various things are "managed" by the managers, create a global to hold the views so we can make references
	var managers []*view.View

	// the chassis power control has a list of 'related items' that we'll accumulate using power_related_items
	var sysChasPwrCtrlVw *view.View
	power_related_items := []string{}

	for _, mgrName := range []string{
		"CMC.Integrated.1",
		"CMC.Integrated.2",
	} {
		//*********************************************************************
		// /redfish/v1/Managers/CMC.Integrated
		//*********************************************************************
		connectTypesSupported := []interface{}{}

		mgrLogger, mgrCmcVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "manager_cmc_integrated",
			map[string]interface{}{
				"rooturi":  rootView.GetURI(),
				"FQDD":     mgrName,                                   // this is used for the AR mapper. case difference is confusing, but need to change mappers
				"fqdd":     "System.Chassis.1#SubSystem.1#" + mgrName, // This is used for the health subsystem
				"fqddlist": []string{mgrName},
			},
		)
		mgrCmcVw.GetModel("default").ApplyOption(
			model.UpdateProperty("connect_types_supported", connectTypesSupported),
			model.UpdateProperty("connect_types_supported_count", len(connectTypesSupported)),
		)

		mgrCmcVw.ApplyOption(
			view.WithModel("global_health", globalHealthModel),

			view.WithModel("health", mgrCmcVw.GetModel("default")), // health info in default model
			view.WithModel("swinv", mgrCmcVw.GetModel("default")),  // common name for swinv model, shared in this case
			view.WithModel("default", mgrCmcVw.GetModel("default")),
			view.WithModel("etag", mgrCmcVw.GetModel("default")),

			view.UpdateEtag("etag", []string{}),

			actionSvc.WithAction(ctx, "manager.reset", "/Actions/Manager.Reset", pumpSvc.NewPumpAction(30)),
			actionSvc.WithAction(ctx, "manager.resettodefaults", "/Actions/Oem/DellManager.ResetToDefaults", pumpSvc.NewPumpAction(30)),
			actionSvc.WithAction(ctx, "manager.forcefailover", "/Actions/Manager.ForceFailover", pumpSvc.NewPumpAction(30)),
			actionSvc.WithAction(ctx, "manager.exportsystemconfig", "/Actions/Oem/EID_674_Manager.ExportSystemConfiguration", exportSystemConfiguration),
			actionSvc.WithAction(ctx, "manager.importsystemconfig", "/Actions/Oem/EID_674_Manager.ImportSystemConfiguration", importSystemConfiguration),
			actionSvc.WithAction(ctx, "manager.importsystemconfigpreview", "/Actions/Oem/EID_674_Manager.ImportSystemConfigurationPreview", importSystemConfigurationPreview),
			actionSvc.WithAction(ctx, "certificates.generatecsr", "/CertificateService/Actions/DellCertificateService.GenerateCSR", pumpSvc.NewPumpAction(30)),
		)

		managers = append(managers, mgrCmcVw)
		swinvViews = append(swinvViews, mgrCmcVw)

		// add the aggregate to the view tree
		mgrCMCIntegrated.AddAggregate(ctx, mgrLogger, mgrCmcVw, ch)
		attributes.AddAggregate(ctx, mgrCmcVw, rootView.GetURI()+"/Managers/"+mgrName+"/Attributes", ch)

		// ######################
		// log related uris
		// ######################
		instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "logservices",
			modParams(map[string]interface{}{"FQDD": mgrName, "collection_uri": baseParams["rooturi"].(string) + "/Managers/" + mgrName + "/LogServices"}),
		)
		instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "lclogservices", modParams(map[string]interface{}{"FQDD": mgrName}))
		instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "faultlistservices", modParams(map[string]interface{}{"FQDD": mgrName}))

		certificate_uris := []string{mgrCmcVw.GetURI() + "/CertificateService/CertificateInventory/FactoryIdentity.1"}

		mgrCmcVw.GetModel("default").ApplyOption(model.UpdateProperty("certificate_uris", certificate_uris))
		certificateservices.AddAggregate(ctx, mgrCmcVw, rootView.GetURI()+"/Managers/"+mgrName, ch)

		// Redundancy
		redundancyLogger, redundancyVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "chassis_cmc_integrated_redundancy",
			map[string]interface{}{
				"rooturi":  rootView.GetURI(),
				"FQDD":     mgrName,                                   // this is used for the AR mapper. case difference is confusing, but need to change mappers
				"fqdd":     "System.Chassis.1#SubSystem.1#" + mgrName, // This is used for the health subsystem
				"fqddlist": []string{mgrName},
			},
		)

		redundancyVw.GetModel("default").ApplyOption(
			model.UpdateProperty("redundancy_set", []string{rootView.GetURI() + "/Managers/CMC.Integrated.1", rootView.GetURI() + "/Managers/CMC.Integrated.2"}),
		)
		redundancy.AddAggregate(ctx, redundancyLogger, redundancyVw, ch)

		// and hook it back into the manager object
		mgrCmcVw.GetModel("default").ApplyOption(
			model.UpdateProperty("redundancy_uris", []string{redundancyVw.GetURI()}),
		)

		//*********************************************************************
		// Create CHASSIS objects for CMC.Integrated.N
		//*********************************************************************
		chasLogger, chasCmcVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "chassis_cmc_integrated",
			map[string]interface{}{
				"rooturi":  rootView.GetURI(),
				"FQDD":     mgrName,                            // this is used for the AR mapper. case difference is confusing, but need to change mappers
				"fqdd":     "System.Chassis.1#SubSystem.1#CMC", // This is used for the health subsystem
				"fqddlist": []string{mgrName},
			},
		)

		chasCmcVw.ApplyOption(
			view.WithModel("etag", chasCmcVw.GetModel("default")),
			view.WithModel("global_health", globalHealthModel),
			view.UpdateEtag("etag", []string{}),
		)

		// add the aggregate to the view tree
		chasCMCIntegrated.AddAggregate(ctx, chasLogger, chasCmcVw, ch)
		attributes.AddAggregate(ctx, chasCmcVw, rootView.GetURI()+"/Chassis/"+mgrName+"/Attributes", ch)

		// add these to the list of related power items
		power_related_items = append(power_related_items, chasCmcVw.GetURI())
	}

	instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "lclogentrycollection",
		modParams(map[string]interface{}{"FQDD": "CMC.Integrated.1", "collection_uri": baseParams["rooturi"].(string) + "/Managers/CMC.Integrated.1/Logs/Lclog"}),
	)
	instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "faultlistentrycollection",
		modParams(map[string]interface{}{"FQDD": "CMC.Integrated.1", "collection_uri": baseParams["rooturi"].(string) + "/Managers/CMC.Integrated.1/Logs/FaultList"}),
	)
	// start log service here: it attaches to cmc.integrated.1
	logSvc.StartService(ctx, logger, managers[0])
	faultSvc.StartService(ctx, logger, managers[0])

	{
		// ************************************************************************
		// CHASSIS System.Chassis.1
		// ************************************************************************
		chasName := "System.Chassis.1"
		sysChasLogger, sysChasVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "system_chassis",
			map[string]interface{}{
				"rooturi":  rootView.GetURI(),
				"FQDD":     chasName,
				"fqddlist": []string{chasName},
			},
		)

		sysChasVw.GetModel("default").ApplyOption(
			model.UpdateProperty("managed_by", []string{managers[0].GetURI()}),
		)

		sysChasVw.ApplyOption(
			view.WithModel("global_health", globalHealthModel),
			actionSvc.WithAction(ctx, "chassis.reset", "/Actions/Chassis.Reset", pumpSvc.NewPumpAction(30)),
			actionSvc.WithAction(ctx, "msmconfigbackup", "/Actions/Oem/MSMConfigBackup", msmConfigBackup),
			actionSvc.WithAction(ctx, "chassis.msmconfigbackup", "/Actions/Oem/DellChassis.MSMConfigBackup", chassisMSMConfigBackup),
		)

		// Create the .../Attributes URI. Attributes are stored in the attributes property of the chasModel
		power_related_items = append(power_related_items, sysChasVw.GetURI())
		system_chassis.AddAggregate(ctx, sysChasLogger, sysChasVw, ch, eb)
		attributes.AddAggregate(ctx, sysChasVw, rootView.GetURI()+"/Chassis/"+chasName+"/Attributes", ch)

		// CMC.INTEGRATED.1 INTERLUDE
		managers[0].GetModel("default").UpdateProperty("manager_for_chassis", []string{sysChasVw.GetURI()})

		//*********************************************************************
		// Create Power objects for System.Chassis.1
		//*********************************************************************
		powerLogger, sysChasPwrVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "power",
			map[string]interface{}{
				"rooturi": rootView.GetURI(),
				"FQDD":    chasName,
			},
		)

		sysChasPwrVw.ApplyOption(
			view.WithModel("global_health", globalHealthModel),
		)
		power.AddAggregate(ctx, powerLogger, sysChasPwrVw, ch)

		psu_uris := []string{}
		for _, psuName := range []string{
			"PSU.Slot.1", "PSU.Slot.2", "PSU.Slot.3",
			"PSU.Slot.4", "PSU.Slot.5", "PSU.Slot.6",
		} {

			psuLogger, sysChasPwrPsuVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "psu_slot",
				map[string]interface{}{
					"rooturi":     rootView.GetURI(),
					"FQDD":        psuName, // this is used for the AR mapper. case difference with 'fqdd' is confusing, but need to change mappers
					"ChassisFQDD": chasName,
					"fqdd":        "System.Chassis.1#" + strings.Replace(psuName, "PSU.Slot", "PowerSupply", 1),
					"fqddlist":    []string{psuName},
				},
			)

			sysChasPwrPsuVw.ApplyOption(
				view.WithModel("swinv", sysChasPwrPsuVw.GetModel("default")),
				view.WithModel("global_health", globalHealthModel),
			)
			swinvViews = append(swinvViews, sysChasPwrPsuVw)
			psu_uris = append(psu_uris, sysChasPwrPsuVw.GetURI())
			powersupply.AddAggregate(ctx, psuLogger, sysChasPwrPsuVw, ch)
		}
		sysChasPwrVw.GetModel("default").ApplyOption(model.UpdateProperty("power_supply_uris", psu_uris))

		// ##################
		// # Power Control
		// ##################

		var pwrCtrlLogger log.Logger
		pwrCtrlLogger, sysChasPwrCtrlVw, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "power_control",
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

		pwrTrendLogger, pwrTrendVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "power_trends",
			map[string]interface{}{
				"rooturi": rootView.GetURI(),
				"FQDD":    chasName,
			},
		)
		powertrends.AddTrendsAggregate(ctx, pwrTrendLogger, pwrTrendVw, ch)
		sysChasPwrVw.GetModel("default").ApplyOption(model.UpdateProperty("power_trends_uri", pwrTrendVw.GetURI()))

		// ##################
		// # Power Histograms
		// ##################

		histogram_uris := []string{}
		for _, trend := range []string{
			"Week", "Day", "Hour",
		} {
			histLogger, histView, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "power_histogram",
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
		thermalLogger, thermalView, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "thermal",
			map[string]interface{}{
				"rooturi": rootView.GetURI(),
				"FQDD":    chasName,
			},
		)

		// thermal_uris := []string{}
		// redundancy_uris := []string{}

		thermalView.ApplyOption(
			view.WithModel("global_health", globalHealthModel),
		)
		thermal.AddAggregate(ctx, thermalLogger, thermalView, ch)

		fan_uris := []string{}
		for _, fanName := range []string{
			"Fan.Slot.1", "Fan.Slot.2", "Fan.Slot.3",
			"Fan.Slot.4", "Fan.Slot.5", "Fan.Slot.6",
			"Fan.Slot.7", "Fan.Slot.8", "Fan.Slot.9",
		} {
			fanLogger, fanView, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "fan",
				map[string]interface{}{
					"rooturi":     rootView.GetURI(),
					"ChassisFQDD": chasName,
					"FQDD":        fanName,
					"fqdd":        "System.Chassis.1#" + fanName,
					"fqddlist":    []string{fanName},
				},
			)

			fanView.ApplyOption(
				view.WithModel("swinv", fanView.GetModel("default")),
				view.WithModel("global_health", globalHealthModel),
			)
			fans.AddAggregate(ctx, fanLogger, fanView, ch)
			fan_uris = append(fan_uris, fanView.GetURI())
			swinvViews = append(swinvViews, fanView)
		}
		thermalView.GetModel("default").ApplyOption(model.UpdateProperty("fan_uris", fan_uris))

		//		thermal_views := []interface{}{}
		//		thermalModel.ApplyOption(model.UpdateProperty("thermal_views", &domain.RedfishResourceProperty{Value: thermal_views}))
		//
		//		redundancy_views := []interface{}{}
		//		thermalModel.ApplyOption(model.UpdateProperty("redundancy_views", &domain.RedfishResourceProperty{Value: redundancy_views}))

		//*********************************************************************
		// Create SubSystemHealth for System.Chassis.1
		//*********************************************************************
		/*subSysHealthLogger := sysChasLogger.New("module", "Chassis/System.Chassis/SubSystemHealth")
		subSysHealthModel := model.New()

		armapper := arService.NewMapping(subSysHealthLogger, "Chassis/"+chasName+"/SubSystemHealth", "Chassis/SubSystemHealths", subSysHealthModel, map[string]string{})

		subSysHealthView := view.New(
			view.WithURI(rootView.GetURI()+"/Chassis/"+chasName+"/SubSystemHealth"),
			view.WithModel("default", subSysHealthModel),
			view.WithController("ar_mapper", armapper),
		)


		subsystemhealth.AddAggregate(ctx, subSysHealthLogger, subSysHealthView, ch, eb)*/
		/* SubSystemHealth */
		subSystemSvc.StartService(ctx, logger, sysChasVw, cfgMgr, instantiateSvc, ch, eb)

		/*  Slots */
		slots.CreateSlotCollection(ctx, sysChasVw, cfgMgr, instantiateSvc, modParams)

		/* Slot config */
		slotconfigSvc.StartService(ctx, logger, sysChasVw, cfgMgr, instantiateSvc, ch, eb)

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
		iomLogger, iomView, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "iom",
			map[string]interface{}{
				"rooturi":  rootView.GetURI(),
				"FQDD":     iomName,
				"fqdd":     "System.Chassis.1#SubSystem.1#" + iomName,
				"fqddlist": []string{iomName},
			},
		)

		iomView.GetModel("default").ApplyOption(
			model.UpdateProperty("managed_by", []string{managers[0].GetURI()}),
		)

		iomView.ApplyOption(
			view.WithModel("swinv", iomView.GetModel("default")),
			view.WithModel("global_health", globalHealthModel),
			actionSvc.WithAction(ctx, "iom.chassis.reset", "/Actions/Chassis.Reset", pumpSvc.NewPumpAction(30)),
			actionSvc.WithAction(ctx, "iom.resetpeakpowerconsumption", "/Actions/Oem/DellChassis.ResetPeakPowerConsumption", pumpSvc.NewPumpAction(30)),
			actionSvc.WithAction(ctx, "iom.virtualreseat", "/Actions/Oem/DellChassis.VirtualReseat", pumpSvc.NewPumpAction(30)),
		)
		swinvViews = append(swinvViews, iomView)
		power_related_items = append(power_related_items, iomView.GetURI())
		iom_chassis.AddAggregate(ctx, iomLogger, iomView, ch, eb)
		attributes.AddAggregate(ctx, iomView, rootView.GetURI()+"/Chassis/"+iomName+"/Attributes", ch)

		// ************************************************************************
		// CHASSIS IOMConfiguration
		// ************************************************************************
		iomCfgLogger, iomCfgView, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "iom_config",
			map[string]interface{}{
				"rooturi":  rootView.GetURI(),
				"FQDD":     iomName,
				"fqdd":     "System.Chassis.1#SubSystem.1#" + iomName,
				"fqddlist": []string{iomName},
			},
		)
		iom_config.AddAggregate(ctx, iomCfgLogger, iomCfgView, ch, eb)
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
		sledLogger, sledView, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "sled",
			map[string]interface{}{
				"rooturi":  rootView.GetURI(),
				"FQDD":     sledName,
				"fqdd":     "System.Chassis.1#SubSystem.1#" + sledName,
				"fqddlist": []string{sledName},
			},
		)

		sledView.GetModel("default").ApplyOption(
			model.UpdateProperty("managed_by", []string{managers[0].GetURI()}),
		)

		sledView.ApplyOption(
			view.WithModel("swinv", sledView.GetModel("default")),
			view.WithModel("global_health", globalHealthModel),
			actionSvc.WithAction(ctx, "chassis.peripheralmapping", "/Actions/Oem/DellChassis.PeripheralMapping", pumpSvc.NewPumpAction(30)),
			actionSvc.WithAction(ctx, "sledvirtualreseat", "/Actions/Chassis.VirtualReseat", pumpSvc.NewPumpAction(30)),
			actionSvc.WithAction(ctx, "chassis.sledvirtualreseat", "/Actions/Oem/DellChassis.VirtualReseat", pumpSvc.NewPumpAction(30)),
		)
		sled_chassis.AddAggregate(ctx, sledLogger, sledView, ch, eb)
		power_related_items = append(power_related_items, sledView.GetURI())
		attributes.AddAggregate(ctx, sledView, rootView.GetURI()+"/Chassis/"+sledName+"/Attributes", ch)
	}

	// link in all of the related items for power control
	sysChasPwrCtrlVw.GetModel("default").ApplyOption(model.UpdateProperty("power_related_items", power_related_items))

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
			actionSvc.WithAction(ctx, "update.reset", "/Actions/Oem/DellUpdateService.Reset", updateReset),
			actionSvc.WithAction(ctx, "update.eid674.reset", "/Actions/Oem/EID_674_UpdateService.Reset", updateEID674Reset),
			actionSvc.WithAction(ctx, "update.syncup", "/Actions/Oem/DellUpdateService.Syncup", pumpSvc.NewPumpAction(30)),
			actionSvc.WithAction(ctx, "update.eid674.syncup", "/Actions/Oem/EID_674_UpdateService.Syncup", pumpSvc.NewPumpAction(30)),
			uploadSvc.WithUpload(ctx, "upload.firmwareUpdate", "/Actions/Oem/FirmwareUpdate", pumpSvc.NewPumpAction(60)),
			evtSvc.PublishResourceUpdatedEventsForModel(ctx, "default"),
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

				// TODO: need this to work at some point...
				//uploadSvc.WithUpload...
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
	self.configChangeHandler = func() {}
	for _, fn := range updateFns {
		fn(ctx, cfgMgr)
	}

	return self
}
