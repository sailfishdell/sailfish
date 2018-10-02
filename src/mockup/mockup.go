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
	"github.com/superchalupa/sailfish/src/ocp/view"
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
	testaggregate.RunRegistryFunctions()
	ar_mapper2.RunRegistryFunctions(arService)

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
	testLogger, testView, err := testaggregate.InstantiateFromCfg(ctx, logger, cfgMgr, "testview", map[string]interface{}{"rooturi": rootView.GetURI()})
	if err == nil {
		testView.ApplyOption(
			evtSvc.PublishResourceUpdatedEventsForModel(ctx, "default"),
		)
		testaggregate.AddAggregate(ctx, testView, ch)

		// separately, start goroutine to listen for test events and create sub uris
		testaggregate.StartService(ctx, testLogger, cfgMgr, rootView, ch, eb)
	}

	//*********************************************************************
	//  /redfish/v1/{Managers,Chassis,Systems,Accounts}
	//*********************************************************************
	// Add standard collections: Systems, Chassis, Mangers, Accounts
	//
	stdcollections.AddAggregate(ctx, rootView.GetUUID(), rootView.GetURI(), ch)

	//*********************************************************************
	// /redfish/v1/Sessions
	//*********************************************************************
	//
	//sessionLogger := logger.New("module", "SessionService")
	sessionModel := model.New(
		model.UpdateProperty("session_timeout", 30))
	// the controller is what updates the model when ar entries change, also
	// handles patch from redfish
	sessionView := view.New(
		view.WithModel("default", sessionModel),
		view.WithURI(rootView.GetURI()+"/SessionService"))
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
		sessionModel.ApplyOption(model.UpdateProperty("session_timeout", cfgMgr.GetInt("session.timeout")))

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
	sessionModel.AddObserver("viper", func(m *model.Model, updates []model.Update) {
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
