package dell_ec

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sync"

	"github.com/spf13/viper"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/actionhandler"
	ah "github.com/superchalupa/sailfish/src/actionhandler"
	"github.com/superchalupa/sailfish/src/dell-ec/slots"
	"github.com/superchalupa/sailfish/src/dell-resources/ar_mapper2"
	"github.com/superchalupa/sailfish/src/dell-resources/attributes"
	"github.com/superchalupa/sailfish/src/dell-resources/certificateservices"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis/subsystemhealth"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis/thermal/fans"
	"github.com/superchalupa/sailfish/src/dell-resources/logservices"
	"github.com/superchalupa/sailfish/src/dell-resources/logservices/faultlist"
	"github.com/superchalupa/sailfish/src/dell-resources/logservices/lcl"
	"github.com/superchalupa/sailfish/src/dell-resources/redundancy"
	"github.com/superchalupa/sailfish/src/dell-resources/registries"
	"github.com/superchalupa/sailfish/src/dell-resources/update_service"
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

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, ch eh.CommandHandler, eb eh.EventBus, d *domain.DomainObjects) *ocp {
	logger = logger.New("module", "ec")
	self := &ocp{}

	// These three all set up a waiter for the root service to appear, so init root service after.
	actionhandler.Setup(ctx, ch, eb)
	uploadhandler.Setup(ctx, ch, eb)
	event.Setup(ch, eb)
	logSvc := lcl.New(ch, eb)
	faultSvc := faultlist.New(ch, eb)
	domain.StartInjectService(logger, eb)
	arService, _ := ar_mapper2.StartService(ctx, logger, cfgMgr, cfgMgrMu, eb)
	actionSvc := ah.StartService(ctx, logger, ch, eb)
	uploadSvc := uploadhandler.StartService(ctx, logger, ch, eb)
	am2Svc, _ := awesome_mapper2.StartService(ctx, logger, eb)
	ardumpSvc, _ := attributes.StartService(ctx, logger, eb)
	telemetryservice.Setup(ctx, actionSvc, ch, eb)
	pumpSvc := NewPumpActionSvc(ctx, logger, eb)

	subSystemSvc := subsystemhealth.New(ch, eb)

	// the package for this is going to change, but this is what makes the various mappers and view functions available
	instantiateSvc := testaggregate.New(ctx, logger, cfgMgr, cfgMgrMu, ch)
	evtSvc := eventservice.New(ctx, cfgMgr, cfgMgrMu, instantiateSvc, actionSvc, ch, eb)
	testaggregate.RegisterWithURI(instantiateSvc)
	testaggregate.RegisterPublishEvents(instantiateSvc, evtSvc)
	testaggregate.RegisterAggregate(instantiateSvc)
	testaggregate.RegisterAM2(instantiateSvc, am2Svc)
	testaggregate.RegisterPumpAction(instantiateSvc, actionSvc, pumpSvc)
	ar_mapper2.RegisterARMapper(instantiateSvc, arService)
	attributes.RegisterController(instantiateSvc, ardumpSvc)
	stdmeta.RegisterFormatters(instantiateSvc, d)
	registries.RegisterAggregate(instantiateSvc)
	stdcollections.RegisterAggregate(instantiateSvc)
	session.RegisterAggregate(instantiateSvc)
	eventservice.RegisterAggregate(instantiateSvc)
	slots.RegisterAggregate(instantiateSvc)
	logservices.RegisterAggregate(instantiateSvc)
	attributes.RegisterAggregate(instantiateSvc)
	fans.RegisterAggregate(instantiateSvc)

	RegisterAggregate(instantiateSvc)
	RegisterIOMAggregate(instantiateSvc)
	RegisterSledAggregate(instantiateSvc)
	RegisterThermalAggregate(instantiateSvc)
	RegisterCMCAggregate(instantiateSvc)
	AddECInstantiate(logger, instantiateSvc)

	// add mapper helper to instantiate
	awesome_mapper2.AddFunction("instantiate", func(args ...interface{}) (interface{}, error) {
		if len(args) < 1 {
			return nil, errors.New("need to specify cfg section to instantiate")
		}
		cfgStr, ok := args[0].(string)
		if !ok {
			return nil, errors.New("need to specify cfg section to instantiate")
		}

		params := map[string]interface{}{}
		var key string
		for i, val := range args[1:] {
			if i%2 == 0 {
				key, ok = val.(string)
				if !ok {
					return nil, fmt.Errorf("got a non-string key value: %s", key)
				}
			} else {
				params[key] = val
			}
		}

		// have to do this in a goroutine because awesome mapper is locked while it processes events
		instantiateSvc.WorkQueue <- func() { instantiateSvc.InstantiateNoWait(cfgStr, params) }
		return true, nil
	})

	//HEALTH
	// The following model maps a bunch of health related stuff that can be tracked once at a global level.
	// we can add this model to the views that need to expose it
	globalHealthModel := model.New()
	healthLogger := logger.New("module", "health_rollup")
	am2Svc.NewMapping(ctx, healthLogger, cfgMgr, cfgMgrMu, globalHealthModel, "global_health", "global_health", map[string]interface{}{})

	//*********************************************************************
	//  /redfish/v1
	//*********************************************************************
	rooturi := "/redfish/v1"
	_, rootView, _ := instantiateSvc.Instantiate("rootview",
		map[string]interface{}{
			"rooturi":           rooturi,
			"globalHealthModel": globalHealthModel,
		})

	//*********************************************************************
	// /redfish/v1/Sessions
	//*********************************************************************
	_, sessionSvcVw, _ := instantiateSvc.Instantiate("sessionservice", map[string]interface{}{})
	session.SetupSessionService(instantiateSvc, sessionSvcVw, ch, eb)
	instantiateSvc.Instantiate("sessioncollection", map[string]interface{}{})

	//*********************************************************************
	//  Standard redfish roles
	//*********************************************************************
	stdcollections.AddStandardRoles(ctx, rootView.GetUUID(), rootView.GetURI(), ch)

	//*********************************************************************
	// /redfish/v1/EventService
	// /redfish/v1/TelemetryService
	//*********************************************************************
	evtSvc.StartEventService(ctx, logger, instantiateSvc, map[string]interface{}{})
	telemetryservice.StartTelemetryService(ctx, logger, rootView)

	//*********************************************************************
	// /redfish/v1/Registries
	//*********************************************************************
	for regName, location := range map[string]interface{}{
		"idrac_registry":    []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/Messages/EEMIRegistry.v1_5_0.json"}},
		"base_registry":     []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/BaseMessages/BaseRegistry.v1_0_0.json", "PublicationUri": "http://www.dmtf.org/sites/default/files/standards/documents/DSP8011_1.0.0a.json"}},
		"mgr_attr_registry": []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/ManagerAttributeRegistry/ManagerAttributeRegistry.v1_0_0.json"}},
	} {
		instantiateSvc.Instantiate(regName, map[string]interface{}{"location": location})
	}

	// various things are "managed" by the managers, create a global to hold the views so we can make references
	var managers []*view.View

	for _, mgrName := range []string{
		"CMC.Integrated.1",
		"CMC.Integrated.2",
	} {
		//*********************************************************************
		// /redfish/v1/Managers/CMC.Integrated
		//*********************************************************************
		_, mgrCmcVw, _ := instantiateSvc.Instantiate("manager_cmc_integrated",
			map[string]interface{}{
				"FQDD": mgrName,
				"exportSystemConfiguration":        view.Action(exportSystemConfiguration),
				"importSystemConfiguration":        view.Action(importSystemConfiguration),
				"importSystemConfigurationPreview": view.Action(importSystemConfigurationPreview),
			},
		)

		managers = append(managers, mgrCmcVw)

		certificate_uris := []string{mgrCmcVw.GetURI() + "/CertificateService/CertificateInventory/FactoryIdentity.1"}

		mgrCmcVw.GetModel("default").ApplyOption(model.UpdateProperty("certificate_uris", certificate_uris))
		certificateservices.AddAggregate(ctx, mgrCmcVw, rootView.GetURI()+"/Managers/"+mgrName, ch)

		// Redundancy
		redundancyLogger, redundancyVw, _ := instantiateSvc.Instantiate("chassis_cmc_integrated_redundancy",
			map[string]interface{}{
				"FQDD": mgrName,
			},
		)

		redundancy_set := []string{rootView.GetURI() + "/Managers/CMC.Integrated.1", rootView.GetURI() + "/Managers/CMC.Integrated.2"}

		redundancyVw.GetModel("default").ApplyOption(
			model.UpdateProperty("redundancy_set", redundancy_set),
		)
		redundancy.AddAggregate(ctx, redundancyLogger, redundancyVw, ch)

		// and hook it back into the manager object
		mgrCmcVw.GetModel("default").ApplyOption(
			model.UpdateProperty("redundancy_uris", []string{redundancyVw.GetURI()}),
		)

		//*********************************************************************
		// Create CHASSIS objects for CMC.Integrated.N
		//*********************************************************************
		instantiateSvc.Instantiate("chassis_cmc_integrated", map[string]interface{}{"FQDD": mgrName})
	}

	// start log service here: it attaches to cmc.integrated.1
	logSvc.StartService(ctx, logger, managers[0])
	faultSvc.StartService(ctx, logger, managers[0])

	{
		// ************************************************************************
		// CHASSIS System.Chassis.1
		// ************************************************************************
		chasName := "System.Chassis.1"
		_, sysChasVw, _ := instantiateSvc.Instantiate("system_chassis",
			map[string]interface{}{
				"FQDD":                   chasName,
				"msmConfigBackup":        view.Action(msmConfigBackup),
				"chassisMSMConfigBackup": view.Action(chassisMSMConfigBackup),
			},
		)

		subSystemSvc.StartService(ctx, logger, sysChasVw, cfgMgr, cfgMgrMu, instantiateSvc)

		// the rest of power uris are automatically created. need to add an awesome mapper function for FindMatchingURIs to migrate this one
		instantiateSvc.Instantiate("power_control",
			map[string]interface{}{
				"FQDD":                chasName,
				"power_related_items": d.FindMatchingURIs(func(uri string) bool { return path.Dir(uri) == rooturi+"/Chassis" }),
			})
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
			actionSvc.WithAction(ctx, "update.reset", "/Actions/Oem/DellUpdateService.Reset", updateReset),
			actionSvc.WithAction(ctx, "update.eid674.reset", "/Actions/Oem/EID_674_UpdateService.Reset", updateEID674Reset),
			actionSvc.WithAction(ctx, "update.syncup", "/Actions/Oem/DellUpdateService.Syncup", pumpSvc.NewPumpAction(30)),
			actionSvc.WithAction(ctx, "update.eid674.syncup", "/Actions/Oem/EID_674_UpdateService.Syncup", pumpSvc.NewPumpAction(30)),
			uploadSvc.WithUpload(ctx, "upload.firmwareUpdate", "/FirmwareInventory", pumpSvc.NewPumpAction(60)),
			evtSvc.PublishResourceUpdatedEventsForModel(ctx, "default"),
		)

		// add the aggregate to the view tree
		update_service.AddAggregate(ctx, rootView, updSvcVw, ch)
		update_service.EnhanceAggregate(ctx, updSvcVw, rootView, ch)
	}

	// VIPER Config:
	// pull the config from the YAML file to populate some static config options
	self.configChangeHandler = func() {}

	return self
}
