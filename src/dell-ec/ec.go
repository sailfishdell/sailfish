package dell_ec

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sync"

	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/actionhandler"
	"github.com/superchalupa/sailfish/src/dell-ec/slots"
	"github.com/superchalupa/sailfish/src/dell-resources/ar_mapper2"
	"github.com/superchalupa/sailfish/src/dell-resources/attributes"
	"github.com/superchalupa/sailfish/src/dell-resources/chassis/system.chassis/thermal/fans"
	"github.com/superchalupa/sailfish/src/dell-resources/logservices"
	"github.com/superchalupa/sailfish/src/dell-resources/registries"
	"github.com/superchalupa/sailfish/src/dell-resources/task_service"
	"github.com/superchalupa/sailfish/src/dell-resources/update_service"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
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
	"github.com/superchalupa/sailfish/godefs"
	_ "github.com/superchalupa/sailfish/src/dell-resources/dm_event"
)

type ocp struct {
	configChangeHandler func()
}

func (o *ocp) ConfigChangeHandler() { o.configChangeHandler() }

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, d *domain.DomainObjects) *ocp {

	logger = log.With(logger, "module", "ec")
	self := &ocp{}
	ch := d.CommandHandler
	eb := d.EventBus

	actionhandler.Setup(ctx, ch, eb)
	uploadhandler.Setup(ctx, ch, eb)
	arService, _ := ar_mapper2.StartService(ctx, logger, cfgMgr, cfgMgrMu, d)
	actionSvc := actionhandler.StartService(ctx, logger, d)
	uploadSvc := uploadhandler.StartService(ctx, logger, d)
	am2Svc, _ := awesome_mapper2.StartService(ctx, logger, eb, ch, d)
	am3Svc, _ := am3.StartService(ctx, logger, "am3 base service", d)
	addAM3Functions(log.With(logger, "module", "ec_am3_functions"), am3Svc, d, ctx)

	// here introduce new initial event handling
	ardumpSvc, _ := attributes.StartService(ctx, logger, d)
	pumpSvc := NewPumpActionSvc(ctx, logger, d)

	// the package for this is going to change, but this is what makes the various mappers and view functions available
	instantiateSvc := testaggregate.New(logger, cfgMgr, cfgMgrMu, d, am3Svc)
	evtSvc := eventservice.New(ctx, logger, cfgMgr, cfgMgrMu, d, instantiateSvc, actionSvc, uploadSvc)

	testaggregate.RegisterWithURI(instantiateSvc)
	testaggregate.RegisterPublishEvents(instantiateSvc, evtSvc)
	testaggregate.RegisterAM2(instantiateSvc, am2Svc)
	testaggregate.RegisterPumpAction(instantiateSvc, actionSvc, pumpSvc)
	testaggregate.RegisterPumpUpload(instantiateSvc, uploadSvc, pumpSvc)

	ar_mapper2.RegisterARMapper(instantiateSvc, arService)
	attributes.RegisterController(instantiateSvc, ardumpSvc)
	stdmeta.RegisterFormatters(instantiateSvc, d)
	stdmeta.SledPwrOperationsSvc(ctx, logger, d)
	registries.RegisterAggregate(instantiateSvc)
	session.RegisterAggregate(instantiateSvc)
	eventservice.RegisterAggregate(instantiateSvc)
	slots.RegisterAggregate(instantiateSvc)
	logservices.RegisterAggregate(instantiateSvc)
	attributes.RegisterAggregate(instantiateSvc)
	update_service.RegisterAggregate(instantiateSvc)
	task_service.RegisterAggregate(instantiateSvc)
	task_service.InitTask(logger, instantiateSvc, am3Svc, ch, ctx)
	fans.RegisterAggregate(instantiateSvc)
	RegisterAggregate(instantiateSvc)
	RegisterIOMAggregate(instantiateSvc)
	RegisterSledAggregate(instantiateSvc)
	RegisterThermalAggregate(instantiateSvc)
	RegisterCMCAggregate(instantiateSvc)
	RegisterCertAggregate(instantiateSvc)
	AddECInstantiate(logger, instantiateSvc)
	initLCL(logger, instantiateSvc, am3Svc, ch, d)
	inithealth(ctx, logger, ch, d)
	stdmeta.InitializeSsoinfo(d) //remove when ready

	stdmeta.SetupSledProfilePlugin(d)
	stdmeta.InitializeCertInfo(d)
	stdmeta.GenericDefPlugin(ch, d)

	// telemetry service is up before aggs/*json are executed
	telemetryservice.New(ctx, logger, ch, d)

	godefs.InitGoDef()

	// start background tree checking
	// disable for now
	//go d.CheckTree()

	awesome_mapper2.AddFunction("find_uris_with_basename", func(args ...interface{}) (interface{}, error) {
		if len(args) < 1 {
			return nil, errors.New("need to specify uri to match")
		}
		p, ok := args[0].(string)
		if !ok {
			return nil, errors.New("need to specify uri to match")
		}

		return d.FindMatchingURIs(func(uri string) bool { return path.Dir(uri) == p }), nil
	})

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

		instantiateSvc.InstantiateNoRet(cfgStr, params)
		return true, nil
	})

	//HEALTH
	// The following model maps a bunch of health related stuff that can be tracked once at a global level.
	// we can add this model to the views that need to expose it
	globalHealthModel := model.New()
	healthLogger := log.With(logger, "module", "health_rollup")
	am2Svc.NewMapping(ctx, healthLogger, cfgMgr, cfgMgrMu, globalHealthModel, "global_health", "global_health", map[string]interface{}{}, nil)

	//*********************************************************************
	//  /redfish/v1
	//*********************************************************************
	_, rootView, _ := instantiateSvc.Instantiate("rootview",
		map[string]interface{}{
			"rooturi":                   "/redfish/v1",
			"globalHealthModel":         globalHealthModel,
			"submit_test_metric_report": view.Action(telemetryservice.MakeSubmitTestMetricReport(eb, d, ch)),
		})

	//*********************************************************************
	// /redfish/v1/Sessions
	//*********************************************************************
	svcLogger, sessionSvcVw, _ := instantiateSvc.Instantiate("sessionservice", map[string]interface{}{})
	session.SetupSessionService(svcLogger, instantiateSvc, sessionSvcVw, d)
	instantiateSvc.InstantiateNoRet("sessioncollection", map[string]interface{}{})

	//*********************************************************************
	//  Standard redfish roles
	//*********************************************************************
	stdcollections.AddStandardRoles(ctx, rootView.GetUUID(), rootView.GetURI(), ch)

	//*********************************************************************
	// /redfish/v1/EventService
	//*********************************************************************
	evtSvc.StartEventService(ctx, instantiateSvc, map[string]interface{}{})

	//*********************************************************************
	// /redfish/v1/Registries
	//*********************************************************************
	for regName, location := range map[string]interface{}{
		"idrac_registry":    []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/Messages/EEMIRegistry.v1_5_0.json"}},
		"base_registry":     []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/BaseMessages/BaseRegistry.v1_0_0.json", "PublicationUri": "http://www.dmtf.org/sites/default/files/standards/documents/DSP8011_1.0.0a.json"}},
		"mgr_attr_registry": []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/ManagerAttributeRegistry/ManagerAttributeRegistry.v1_0_0.json"}},
	} {
		instantiateSvc.InstantiateNoRet(regName, map[string]interface{}{"location": location})
	}

	_, updSvcVw, _ := instantiateSvc.Instantiate("update_service", map[string]interface{}{})

	updSvcVw.ApplyOption(
		uploadSvc.WithUpload(ctx, "upload.firmwareUpdate", "/FirmwareInventory", pumpSvc.NewPumpAction(300)),
	)

	// VIPER Config:
	// pull the config from the YAML file to populate some static config options
	self.configChangeHandler = func() {}

	// send startup events
	setup(logger, d)

	return self
}
