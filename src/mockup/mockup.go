package mockup

import (
	"context"
	"sync"

	"io/ioutil"

	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/actionhandler"
	"github.com/superchalupa/sailfish/src/dell-resources/ar_mapper2"
	"github.com/superchalupa/sailfish/src/eventwaiter"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/event"
	"github.com/superchalupa/sailfish/src/ocp/eventservice"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/root"
	"github.com/superchalupa/sailfish/src/ocp/session"
	"github.com/superchalupa/sailfish/src/ocp/stdcollections"
	"github.com/superchalupa/sailfish/src/ocp/telemetryservice"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

type ocp struct {
	configChangeHandler func()
}

func (o *ocp) ConfigChangeHandler() { o.configChangeHandler() }

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
}

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.Mutex, ch eh.CommandHandler, eb eh.EventBus, ew waiter) *ocp {
	logger = logger.New("module", "ec")
	self := &ocp{}

	updateFns := []func(context.Context, *viper.Viper){}

	// service startup
	domain.StartInjectService(eb)
	actionhandler.Setup(ctx, ch, eb)
	evtSvc := eventservice.New(ctx, ch, eb)
	telemetryservice.Setup(ctx, ch, eb)
	event.Setup(ch, eb)

	arService, _ := ar_mapper2.StartService(ctx, logger, eb)
	updateFns = append(updateFns, arService.ConfigChangedFn)

	// the package for this is going to change, but this is what makes the various mappers and view functions available
	instantiateSvc := testaggregate.New(logger, ch)
	testaggregate.RegisterWithURI(instantiateSvc)
	testaggregate.RegisterPublishEvents(instantiateSvc, evtSvc)
	ar_mapper2.RegisterARMapper(instantiateSvc, arService)

	//
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
	_, testView, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "testview", map[string]interface{}{"rooturi": rootView.GetURI()})
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

	// VIPER Config:
	// pull the config from the YAML file to populate some static config options
	self.configChangeHandler = func() {
		logger.Info("Re-applying configuration from config file.")
		sessionView.GetModel("default").UpdateProperty("session_timeout", cfgMgr.GetInt("session.timeout"))

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
