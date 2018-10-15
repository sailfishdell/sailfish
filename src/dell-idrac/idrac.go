package dell_idrac

import (
	"context"
	"fmt"
	"sync"

	"github.com/spf13/viper"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/actionhandler"
	ah "github.com/superchalupa/sailfish/src/actionhandler"
	"github.com/superchalupa/sailfish/src/dell-resources/ar_mapper2"
	"github.com/superchalupa/sailfish/src/dell-resources/attributes"
	system_chassis "github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis"
	"github.com/superchalupa/sailfish/src/dell-resources/registries"
	storage_enclosure "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/enclosure"
	"github.com/superchalupa/sailfish/src/eventwaiter"
	"github.com/superchalupa/sailfish/src/log"
	am2 "github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/event"
	"github.com/superchalupa/sailfish/src/ocp/eventservice"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/root"
	"github.com/superchalupa/sailfish/src/ocp/session"
	"github.com/superchalupa/sailfish/src/ocp/stdcollections"
	"github.com/superchalupa/sailfish/src/ocp/telemetryservice"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"

	//"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
	"github.com/superchalupa/sailfish/src/stdmeta"

	// register all the DM events that are not otherwise pulled in
	_ "github.com/superchalupa/sailfish/src/dell-resources/dm_event"

	// goal is to get rid of the _ in front of each of these....
	_ "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage"
	_ "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/controller"
	_ "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/drive"
	_ "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/storage_collection"
	_ "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/volume"
	_ "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/volume_collection"
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

	// These three all set up a waiter for the root service to appear, so init root service after.
	actionhandler.Setup(ctx, ch, eb)
	evtSvc := eventservice.New(ctx, ch, eb)
	telemetryservice.Setup(ctx, ch, eb)
	event.Setup(ch, eb)

	domain.StartInjectService(eb)

	arService, _ := ar_mapper2.StartService(ctx, logger, eb)
	updateFns = append(updateFns, arService.ConfigChangedFn)

	// the package for this is going to change, but this is what makes the various mappers and view functions available
	instantiateSvc := testaggregate.New(logger, ch)
	testaggregate.RegisterWithURI(instantiateSvc)
	testaggregate.RegisterPublishEvents(instantiateSvc, evtSvc)

	ar_mapper2.RegisterARMapper(instantiateSvc, arService)
	attributes.RegisterARMapper(instantiateSvc, ch, eb)
	stdmeta.RegisterFormatters(instantiateSvc, d)
	registries.RegisterAggregate(instantiateSvc)

	// awesome mapper 2 service
	am2Svc, err := am2.StartService(ctx, logger, eb)
	if err != nil {
		logger.Error("Failed to start awesome mapper 2", "err", err)
	}
	testaggregate.RegisterAM2(instantiateSvc, am2Svc)

	// Create the (empty) model behind the /redfish/v1 service root. Nothing interesting here
	//
	// No Logger
	// No Model
	// No Controllers
	// View created so we have a place to hold the aggregate UUID and URI
	_, rootView, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "rootview", map[string]interface{}{})
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
	_, testView, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "testview", map[string]interface{}{"rooturi": rootView.GetURI(), "fqdd": "System.Modular.1"})
	testaggregate.AddAggregate(ctx, testView, ch)

	//*********************************************************************
	//  /redfish/v1/{Managers,Chassis,Systems,Accounts}
	//*********************************************************************
	baseParams := map[string]interface{}{"rooturi": rootView.GetURI(), "rootid": rootView.GetUUID(), "collection_uri": "/redfish/v1/Chassis"}
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
	_, sessionView, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "sessionview", map[string]interface{}{"rooturi": rootView.GetURI()})
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
	instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "registries", map[string]interface{}{"rooturi": rootView.GetURI()})

	for regName, location := range map[string]interface{}{
		"idrac_registry":    []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/Messages/EEMIRegistry.v1_5_0.json"}},
		"base_registry":     []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/BaseMessages/BaseRegistry.v1_0_0.json", "PublicationUri": "http://www.dmtf.org/sites/default/files/standards/documents/DSP8011_1.0.0a.json"}},
		"mgr_attr_registry": []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/ManagerAttributeRegistry/ManagerAttributeRegistry.v1_0_0.json"}},
	} {
		instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, regName, map[string]interface{}{"location": location, "rooturi": rootView.GetURI()})
	}

	// storage_enclosure_items := []string{}

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

		sysChasVw.ApplyOption(
			ah.WithAction(ctx, sysChasLogger, "chassis.reset", "/Actions/Chassis.Reset", makePumpHandledAction("ChassisReset", 30, eb), ch, eb),
		)

		// Create the .../Attributes URI. Attributes are stored in the attributes property of the chasModel
		system_chassis.AddAggregate(ctx, sysChasLogger, sysChasVw, ch, eb)
		attributes.AddAggregate(ctx, sysChasVw, rootView.GetURI()+"/Chassis/"+chasName+"/Attributes", ch)

		// ##################
		// # Storage Enclosure
		// ##################
		fmt.Printf("Startup for enclosure")

		strgCntlrLogger, sysStorEnclsrCtrlVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "storage_enclosure",
			map[string]interface{}{
				"rooturi": rootView.GetURI(),
				"FQDD":    "test.fqdd",
			},
		)

		storage_enclosure.AddAggregate(ctx, strgCntlrLogger, sysStorEnclsrCtrlVw, ch)
		// storage_enclosure_items = append(storage_enclosure_items, sysStorEnclsrCtrlVw.GetURI())
		//sysStorEnclsrCtrlVw.GetModel("default").ApplyOption(model.UpdateProperty("link_uris", storage_enclosure_items))

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

	return self
}
