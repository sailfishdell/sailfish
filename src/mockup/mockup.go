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
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/event"
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

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
}

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, ch eh.CommandHandler, eb eh.EventBus, d *domain.DomainObjects) {
	logger = logger.New("module", "ec")

	// These three all set up a waiter for the root service to appear, so init root service after.
	actionhandler.Setup(ctx, ch, eb)
	event.Setup(ch, eb)
	domain.StartInjectService(logger, d)
	actionSvc := ah.StartService(ctx, logger, ch, eb)
	uploadSvc := uploadhandler.StartService(ctx, logger, ch, eb)
	am2Svc, _ := awesome_mapper2.StartService(ctx, logger, eb, ch, d)
	pumpSvc := dell_ec.NewPumpActionSvc(ctx, logger, eb)

	// the package for this is going to change, but this is what makes the various mappers and view functions available
	instantiateSvc := testaggregate.New(ctx, logger, cfgMgr, cfgMgrMu, ch)
	evtSvc := eventservice.New(ctx, cfgMgr, cfgMgrMu, d, instantiateSvc, actionSvc, uploadSvc)
	eventservice.RegisterAggregate(instantiateSvc)
	testaggregate.RegisterWithURI(instantiateSvc)
	testaggregate.RegisterPublishEvents(instantiateSvc, evtSvc)
	testaggregate.RegisterAM2(instantiateSvc, am2Svc)
	testaggregate.RegisterPumpAction(instantiateSvc, actionSvc, pumpSvc)
	registries.RegisterAggregate(instantiateSvc)
	stdmeta.RegisterFormatters(instantiateSvc, d)
	stdcollections.RegisterAggregate(instantiateSvc)
	session.RegisterAggregate(instantiateSvc)
	telemetryservice.RegisterAggregate(instantiateSvc)

	stdmeta.GenericDefPlugin(ch, d)

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

		instantiateSvc.Instantiate(cfgStr, params)
		return true, nil
	})

	// ignore unused for now
	_ = actionSvc

	//*********************************************************************
	//  /redfish/v1
	//*********************************************************************
	_, rootView, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "rootview", map[string]interface{}{
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
	_, sessionSvcVw, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "sessionservice", map[string]interface{}{})
	session.SetupSessionService(instantiateSvc, sessionSvcVw, ch, eb)
	instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "sessioncollection", map[string]interface{}{"collection_uri": "/redfish/v1/SessionService/Sessions"})

	//*********************************************************************
	// /redfish/v1/EventService
	//*********************************************************************
	evtSvc.StartEventService(ctx, logger, instantiateSvc, map[string]interface{}{})
}
