package dell_msm

import (
	"context"
	"sync"

	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/actionhandler"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
	"github.com/superchalupa/sailfish/src/ocp/eventservice"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
	"github.com/superchalupa/sailfish/src/stdmeta"
	"github.com/superchalupa/sailfish/src/uploadhandler"
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
	am3Svc, _ := am3.StartService(ctx, logger, "am3 base service", d)

	// here introduce new initial event handling
	actionSvc := actionhandler.StartService(ctx, logger, d)
	uploadSvc := uploadhandler.StartService(ctx, logger, d)
	pumpSvc := NewPumpActionSvc(ctx, logger, d)

	instantiateSvc := testaggregate.New(logger, cfgMgr, cfgMgrMu, d, am3Svc)

	testaggregate.RegisterWithURI(instantiateSvc)
	stdmeta.RegisterFormatters(instantiateSvc, d)
	testaggregate.RegisterPumpAction(instantiateSvc, actionSvc, pumpSvc)

	eventservice.RegisterAggregate(instantiateSvc)

	evtSvc := eventservice.New(ctx, logger, cfgMgr, cfgMgrMu, d, instantiateSvc, actionSvc, uploadSvc)
	testaggregate.RegisterPublishEvents(instantiateSvc, evtSvc)

	//*********************************************************************
	// /redfish/v1/EventService
	//*********************************************************************
	evtSvc.StartEventService(ctx, instantiateSvc, map[string]interface{}{})

	// VIPER Config:
	// pull the config from the YAML file to populate some static config options
	self.configChangeHandler = func() {}

	return self
}
