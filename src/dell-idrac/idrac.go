package dell_idrac

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/spf13/viper"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/actionhandler"
	ah "github.com/superchalupa/sailfish/src/actionhandler"
	"github.com/superchalupa/sailfish/src/dell-resources/ar_mapper2"
	"github.com/superchalupa/sailfish/src/dell-resources/attributes"
	"github.com/superchalupa/sailfish/src/dell-resources/logservices/faultlist"
	"github.com/superchalupa/sailfish/src/dell-resources/logservices/lcl"
	"github.com/superchalupa/sailfish/src/dell-resources/registries"
	"github.com/superchalupa/sailfish/src/eventwaiter"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/event"
	"github.com/superchalupa/sailfish/src/ocp/eventservice"
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
	"github.com/superchalupa/sailfish/src/dell-idrac/chassis/system_chassis"
	"github.com/superchalupa/sailfish/src/dell-idrac/managers/idrac_embedded"
	"github.com/superchalupa/sailfish/src/dell-idrac/storage"
	/*
		// all these are TODO:
				storage_enclosure "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/enclosure"
				storage_instance "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage"
				storage_controller "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/controller"
				storage_drive "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/drive"
				storage_collection "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/storage_collection"
				storage_volume "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/volume"
				storage_volume_collection "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/volume_collection"
	*/)

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
	attributes.RegisterAggregate(instantiateSvc)
	eventservice.RegisterAggregate(instantiateSvc)
	idrac_embedded.RegisterAggregate(instantiateSvc)
	system_chassis.RegisterAggregate(instantiateSvc)
	storage.RegisterAggregate(instantiateSvc)

	AddChassisInstantiate(logger, instantiateSvc)

	// storage
	//storage_collection.RegisterAggregate(instantiateSvc)
	//storage_volume_collection.RegisterAggregate(instantiateSvc)

	// ignore unused for now
	_ = logSvc
	_ = faultSvc
	_ = actionSvc

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

	//*********************************************************************
	//  /redfish/v1
	//*********************************************************************
	rooturi := "/redfish/v1"
	_, rootView, _ := instantiateSvc.Instantiate("rootview",
		map[string]interface{}{
			"rooturi": rooturi,
		})

	//*********************************************************************
	//  Standard redfish roles
	//*********************************************************************
	// TODO: convert to instantiate
	stdcollections.AddStandardRoles(ctx, rootView.GetUUID(), rootView.GetURI(), ch)

	//*********************************************************************
	// /redfish/v1/Sessions
	//*********************************************************************
	_, sessionSvcVw, _ := instantiateSvc.Instantiate("sessionservice", map[string]interface{}{})
	session.SetupSessionService(instantiateSvc, sessionSvcVw, ch, eb)
	instantiateSvc.Instantiate("sessioncollection", map[string]interface{}{})

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

	//*********************************************************************
	// /redfish/v1/Managers/iDRAC.Embedded.1
	//*********************************************************************
	instantiateSvc.Instantiate("idrac_embedded", map[string]interface{}{"FQDD": "iDRAC.Embedded.1"})

	// stuff below is "legacy" and is in-progress for conversion
	/*
		storage_enclosure_items := []string{}
		storage_instance_items := []string{}
		storage_controller_items := []string{}
		storage_drive_items := []string{}
		storage_vol_items := []string{}

		{

			// ##################
			// # Storage Enclosure
			// ##################
			fmt.Printf("Startup for enclosure\n")

			strgCntlrLogger, sysStorEnclsrCtrlVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "storage_enclosure",
				map[string]interface{}{
					"rooturi":    rootView.GetURI(),
					"URI_FQDD":   "Enclosure.Internal.0-1:RAID.Slot.4-1",
					"EVENT_FQDD": "308|C|Enclosure.Internal.0-1:RAID.Slot.4-1",
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
					"rooturi":    rootView.GetURI(),
					"URI_FQDD":   "AHCI.Embedded.2-1",
					"EVENT_FQDD": "301|P|AHCI.Embedded.2-1",
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
					"rooturi":    rootView.GetURI(),
					"URI_FQDD":   "RAID.Slot.4-1",
					"EVENT_FQDD": "301|P|AHCI.Embedded.2-1",
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
					"rooturi":    rootView.GetURI(),
					"URI_FQDD":   "Disk.Bay.0:Enclosure.Internal.0-1:RAID.Slot.4-1",
					"EVENT_FQDD": "304|P|Disk.Bay.0:Enclosure.Internal.0-1:RAID.Slot.4-1",
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
					"rooturi":    rootView.GetURI(),
					"URI_FQDD":   "Disk.Virtual.0:RAID.Slot.4-1",
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
	*/

	return self
}
