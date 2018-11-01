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
	"github.com/superchalupa/sailfish/src/dell-resources/logservices/faultlist"
	"github.com/superchalupa/sailfish/src/dell-resources/logservices/lcl"
	"github.com/superchalupa/sailfish/src/dell-resources/registries"
	storage_enclosure "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/enclosure"
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

	//"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
	"github.com/superchalupa/sailfish/src/stdmeta"

	// register all the DM events that are not otherwise pulled in
	_ "github.com/superchalupa/sailfish/src/dell-resources/dm_event"

	// goal is to get rid of the _ in front of each of these....
	dell_ec "github.com/superchalupa/sailfish/src/dell-ec"
	mgrCMCIntegrated "github.com/superchalupa/sailfish/src/dell-resources/managers/cmc.integrated"
	storage_instance "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage"
	storage_controller "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/controller"
	storage_drive "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/drive"
	storage_collection "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/storage_collection"
	storage_volume "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/volume"
	storage_volume_collection "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/volume_collection"
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
	event.Setup(ch, eb)
	logSvc := lcl.New(ch, eb)
	faultSvc := faultlist.New(ch, eb)
	domain.StartInjectService(logger, eb)
	arService, _ := ar_mapper2.StartService(ctx, logger, cfgMgr, cfgMgrMu, eb)
	actionSvc := ah.StartService(ctx, logger, ch, eb)
	am2Svc, _ := awesome_mapper2.StartService(ctx, logger, eb)
	ardumpSvc, _ := attributes.StartService(ctx, logger, eb)
	telemetryservice.Setup(ctx, actionSvc, ch, eb)
	pumpSvc := dell_ec.NewPumpActionSvc(ctx, logger, eb)

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
	storage_collection.RegisterAggregate(instantiateSvc)
	storage_volume_collection.RegisterAggregate(instantiateSvc)

	// ignore unused for now
	_ = logSvc
	_ = faultSvc
	_ = actionSvc

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

	//*********************************************************************
	//  /redfish/v1
	//*********************************************************************
	_, rootView, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "rootview", map[string]interface{}{})
	baseParams["rootid"] = rootView.GetUUID()

	//*********************************************************************
	//  /redfish/v1/testview - a proof of concept test view and example
	//*********************************************************************
	instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "testview", baseParams)

	//*********************************************************************
	//  /redfish/v1/{Managers,Chassis,Systems,Accounts}
	//*********************************************************************
	_, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "chassis", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/Chassis"}))
	_, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "systems", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/Systems"}))
	_, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "managers", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/Managers"}))
	_, accountSvcVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "accountservice", modParams(map[string]interface{}{}))
	baseParams["actsvc_uri"] = accountSvcVw.GetURI()
	baseParams["actsvc_id"] = accountSvcVw.GetUUID()
	_, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "roles", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/AccountService/Roles"}))
	_, _, _ = instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "accounts", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/AccountService/Accounts"}))

	//*********************************************************************
	//  Standard redfish roles
	//*********************************************************************
	stdcollections.AddStandardRoles(ctx, rootView.GetUUID(), rootView.GetURI(), ch)

	//*********************************************************************
	// /redfish/v1/Sessions
	//*********************************************************************
	_, sessionSvcVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "sessionservice", baseParams)
	baseParams["sessionsvc_id"] = sessionSvcVw.GetUUID()
	baseParams["sessionsvc_uri"] = sessionSvcVw.GetURI()
	session.SetupSessionService(instantiateSvc, sessionSvcVw, ch, eb)
	instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "sessioncollection", modParams(map[string]interface{}{"collection_uri": "/redfish/v1/SessionService/Sessions"}))

	//*********************************************************************
	// /redfish/v1/EventService
	// /redfish/v1/TelemetryService
	//*********************************************************************
	evtSvc.StartEventService(ctx, logger, instantiateSvc, baseParams)
	telemetryservice.StartTelemetryService(ctx, logger, rootView)

	//*********************************************************************
	// /redfish/v1/Registries
	//*********************************************************************
	instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "registries", map[string]interface{}{"rooturi": rootView.GetURI()})

	for regName, location := range map[string]interface{}{
		"idrac_registry":    []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/Messages/EEMIRegistry.v1_5_0.json"}},
		"base_registry":     []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/BaseMessages/BaseRegistry.v1_0_0.json", "PublicationUri": "http://www.dmtf.org/sites/default/files/standards/documents/DSP8011_1.0.0a.json"}},
		"mgr_attr_registry": []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/ManagerAttributeRegistry/ManagerAttributeRegistry.v1_0_0.json"}},
	} {
		instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, regName, map[string]interface{}{"location": location, "rooturi": rootView.GetURI()})
	}

	// various things are "managed" by the managers, create a global to hold the views so we can make references
	//	var managers []*view.View

	mgrName := "iDRAC.Embedded.1"

	//*********************************************************************
	// /redfish/v1/Managers/iDRAC.Embedded.1
	//*********************************************************************

	mgrLogger, mgrCmcVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "idrac_embedded",
		map[string]interface{}{
			"rooturi":  rootView.GetURI(),
			"FQDD":     mgrName,                                   // this is used for the AR mapper. case difference is confusing, but need to change mappers
			"fqdd":     "System.Chassis.1#SubSystem.1#" + mgrName, // This is used for the health subsystem
			"fqddlist": []string{mgrName},
		},
	)

	//	managers = append(managers, mgrCmcVw)
	//	swinvViews = append(swinvViews, mgrCmcVw)

	// add the aggregate to the view tree
	mgrCMCIntegrated.AddAggregate(ctx, mgrLogger, mgrCmcVw, ch)
	attributes.AddAggregate(ctx, mgrCmcVw, rootView.GetURI()+"/Managers/"+mgrName+"/Attributes", ch)

	//end

	storage_enclosure_items := []string{}
	storage_instance_items := []string{}
	storage_controller_items := []string{}
	storage_drive_items := []string{}
	storage_vol_items := []string{}

	{
		// ************************************************************************
		// CHASSIS System.Chassis.1
		// ************************************************************************
		chasName := "System.Chassis.1"
		sysChasLogger, sysChasVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "system_chassis",
			map[string]interface{}{
				"rooturi":  rootView.GetURI(),
				"FQDD":     chasName,
				"fqddlist": []string{chasName},
			},
		)

		// Create the .../Attributes URI. Attributes are stored in the attributes property of the chasModel
		system_chassis.AddAggregate(ctx, sysChasLogger, sysChasVw, ch, eb)
		attributes.AddAggregate(ctx, sysChasVw, rootView.GetURI()+"/Chassis/"+chasName+"/Attributes", ch)

		// ##################
		// # Storage Enclosure
		// ##################
		fmt.Printf("Startup for enclosure\n")

		strgCntlrLogger, sysStorEnclsrCtrlVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "storage_enclosure",
			map[string]interface{}{
				"rooturi": rootView.GetURI(),
				"URI_FQDD":    "Enclosure.Internal.0-1:RAID.Slot.4-1",
				"EVENT_FQDD":    "308|C|Enclosure.Internal.0-1:RAID.Slot.4-1",
			},
		)

		storage_enclosure.AddAggregate(ctx, strgCntlrLogger, sysStorEnclsrCtrlVw, ch)
		storage_enclosure_items = append(storage_enclosure_items, sysStorEnclsrCtrlVw.GetURI())
		sysStorEnclsrCtrlVw.GetModel("default").ApplyOption(model.UpdateProperty("link_uris", storage_enclosure_items))

		// ##################
		// # Storage Instance
		// ##################
		fmt.Printf("Startup for Storage instance\n")

		strgInstanceLogger, sysStorInstanceCtrlVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "storage_instance",
			map[string]interface{}{
				"rooturi": rootView.GetURI(),
				"URI_FQDD":    "AHCI.Embedded.2-1",
				"EVENT_FQDD":    "301|P|AHCI.Embedded.2-1",
			},
		)

		storage_instance.AddAggregate(ctx, strgInstanceLogger, sysStorInstanceCtrlVw, ch)
		storage_instance_items = append(storage_instance_items, sysStorInstanceCtrlVw.GetURI())
		sysStorInstanceCtrlVw.GetModel("default").ApplyOption(model.UpdateProperty("link_uris", storage_instance_items))

		// ##################
		// # Storage controller
		// ##################
		fmt.Printf("Startup for Storage controller\n")

		strgControllerLogger, sysStorControllerCtrlVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "storage_controller",
			map[string]interface{}{
				"rooturi": rootView.GetURI(),
				"URI_FQDD":    "RAID.Slot.4-1",
				"EVENT_FQDD":    "301|P|AHCI.Embedded.2-1",
			},
		)

		storage_controller.AddAggregate(ctx, strgControllerLogger, sysStorControllerCtrlVw, ch)
		storage_controller_items = append(storage_controller_items, sysStorControllerCtrlVw.GetURI())
		sysStorControllerCtrlVw.GetModel("default").ApplyOption(model.UpdateProperty("link_uris", storage_controller_items))

		// ##################
		// # Storage drive
		// ##################
		fmt.Printf("Startup for Storage drive\n")

		strgDriveLogger, sysStorDriveCtrlVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "storage_drive",
			map[string]interface{}{
				"rooturi": rootView.GetURI(),
				"URI_FQDD":    "Disk.Bay.0:Enclosure.Internal.0-1:RAID.Slot.4-1",
				"EVENT_FQDD":    "304|P|Disk.Bay.0:Enclosure.Internal.0-1:RAID.Slot.4-1",
			},
		)

		storage_drive.AddAggregate(ctx, strgDriveLogger, sysStorDriveCtrlVw, ch)
		storage_drive_items = append(storage_drive_items, sysStorDriveCtrlVw.GetURI())
		sysStorDriveCtrlVw.GetModel("default").ApplyOption(model.UpdateProperty("link_uris", storage_drive_items))

		// ##################
		// # Storage volume
		// ##################
		fmt.Printf("Startup for Storage volume\n")

		strgVolLogger, sysStorVolCtrlVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "storage_volume",
			map[string]interface{}{
				"rooturi": rootView.GetURI(),
				"URI_FQDD": "Disk.Virtual.0:RAID.Slot.4-1",
				"EVENT_FQDD": "305|P|Disk.Virtual.0:RAID.Slot.4-1",
			},
		)

		storage_volume.AddAggregate(ctx, strgVolLogger, sysStorVolCtrlVw, ch)
		storage_vol_items = append(storage_vol_items, sysStorVolCtrlVw.GetURI())
		sysStorVolCtrlVw.GetModel("default").ApplyOption(model.UpdateProperty("link_uris", storage_vol_items))

		// ##################
		// # Storage volume Collection
		// ##################
		fmt.Printf("Startup for Storage volume collection\n")

//		strgVolCollLogger, sysStorVolCollCtrlVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "storage_volume_collection",
//			map[string]interface{}{
//				"rooturi": rootView.GetURI(),
//				"URI_FQDD":    "RAID.Slot.4-1",
//			},
//		)

//		storage_volume_collection.AddAggregate(ctx, strgVolCollLogger, sysStorVolCollCtrlVw, ch)
		//storage_vol_coll_items = append(storage_vol_coll_items, sysStorVolCollCtrlVw.GetURI())
		//sysStorVolCollCtrlVw.GetModel("default").ApplyOption(model.UpdateProperty("link_uris", storage_vol_coll_items))
		// ##################
		// # Storage Collection
		// ##################
		fmt.Printf("Startup for Storage collection\n")

//		strgCollLogger, sysStorCollCtrlVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "storage_collection",
	//		map[string]interface{}{
	//			"rooturi": rootView.GetURI(),
	//			//"URI_FQDD":    "RAID.Slot.4-1",
	//		},
		//)

		//storage_collection.AddAggregate(ctx, strgCollLogger, sysStorCollCtrlVw, ch)
		//storage_coll_items = append(storage_coll_items, sysStorCollCtrlVw.GetURI())
		//sysStorCollCtrlVw.GetModel("default").ApplyOption(model.UpdateProperty("link_uris", storage_coll_items))
	}

	return self
}
