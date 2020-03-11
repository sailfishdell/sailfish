package mockup

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/spf13/viper"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/actionhandler"
	ah "github.com/superchalupa/sailfish/src/actionhandler"
	dell_ec "github.com/superchalupa/sailfish/src/dell-ec"
	"github.com/superchalupa/sailfish/src/dell-resources/registries"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/eventservice"
	"github.com/superchalupa/sailfish/src/ocp/session"
	"github.com/superchalupa/sailfish/src/ocp/stdcollections"
	"github.com/superchalupa/sailfish/src/ocp/telemetryservice"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
	"github.com/superchalupa/sailfish/src/stdmeta"
	"github.com/superchalupa/sailfish/src/uploadhandler"
)

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, ch eh.CommandHandler, eb eh.EventBus, d *domain.DomainObjects) {
	logger = log.With(logger, "module", "ec")

	actionhandler.Setup(ctx, ch, eb)
	actionSvc := ah.StartService(ctx, logger, d)
	am2Svc, _ := awesome_mapper2.StartService(ctx, logger, eb, ch, d)
	am3Svc, _ := am3.StartService(ctx, logger, "am3 base service", d)
	instantiateSvc := testaggregate.New(logger, cfgMgr, cfgMgrMu, d, am3Svc)
	uploadSvc := uploadhandler.StartService(ctx, logger, d)
	evtSvc := eventservice.New(ctx, cfgMgr, cfgMgrMu, d, instantiateSvc, actionSvc, uploadSvc)
	eventservice.RegisterAggregate(instantiateSvc)
	testaggregate.RegisterWithURI(instantiateSvc)
	testaggregate.RegisterPublishEvents(instantiateSvc, evtSvc)
	testaggregate.RegisterAM2(instantiateSvc, am2Svc)
	registries.RegisterAggregate(instantiateSvc)
	stdmeta.RegisterFormatters(instantiateSvc, d)
	session.RegisterAggregate(instantiateSvc)

	stdmeta.GenericDefPlugin(ch, d)
	pumpSvc := dell_ec.NewPumpActionSvc(ctx, logger, d)
	testaggregate.RegisterPumpAction(instantiateSvc, actionSvc, pumpSvc)

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

	// ignore unused for now
	_ = actionSvc

	//*********************************************************************
	//  /redfish/v1
	//*********************************************************************
	_, rootView, _ := instantiateSvc.Instantiate("rootview", map[string]interface{}{
		"rooturi":                   "/redfish/v1",
		"submit_test_metric_report": view.Action(telemetryservice.MakeSubmitTestMetricReport(eb, d, ch)),
	})

	//*********************************************************************
	//  Standard redfish roles
	//*********************************************************************
	stdcollections.AddStandardRoles(ctx, rootView.GetUUID(), rootView.GetURI(), ch)

	//*********************************************************************
	// /redfish/v1/Sessions
	//*********************************************************************
	_, sessionSvcVw, _ := instantiateSvc.Instantiate("sessionservice", map[string]interface{}{})
	session.SetupSessionService(instantiateSvc, sessionSvcVw, d)
	instantiateSvc.InstantiateNoRet("sessioncollection", map[string]interface{}{"collection_uri": "/redfish/v1/SessionService/Sessions"})

	//*********************************************************************
	// /redfish/v1/EventService
	//*********************************************************************
	evtSvc.StartEventService(ctx, logger, instantiateSvc, map[string]interface{}{})
}
