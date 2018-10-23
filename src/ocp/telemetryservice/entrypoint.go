package telemetryservice

import (
	"context"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/view"
)

type viewer interface {
	GetUUID() eh.UUID
	GetURI() string
}

type actionService interface {
	WithAction(context.Context, string, string, view.Action) view.Option
}

var StartTelemetryService func(context.Context, log.Logger, viewer) *view.View

func Setup(ctx context.Context, actionSvc actionService, ch eh.CommandHandler, eb eh.EventBus) {
	StartTelemetryService = func(ctx context.Context, logger log.Logger, rootView viewer) *view.View {
		return startTelemetryService(ctx, logger, rootView, actionSvc, ch, eb)
	}
}

// StartTelemetryService will create a model, view, and controller for the Telemetryservice, then start a goroutine to publish events
//      If you want to save settings, hook up a mapper to the "default" view returned
func startTelemetryService(ctx context.Context, logger log.Logger, rootView viewer, actionSvc actionService, ch eh.CommandHandler, eb eh.EventBus) *view.View {
	tsLogger := logger.New("module", "TelemetryService")

	tsModel := model.New()

	tsView := view.New(
		view.WithModel("default", tsModel),
		view.WithURI(rootView.GetURI()+"/TelemetryService"),
		actionSvc.WithAction(ctx, "submit.test.metric.report", "/Actions/TelemetryService.SubmitTestMetricReport", MakeSubmitTestMetricReport(eb)),
	)

	// The Plugin: "TelemetryService" property on the Subscriptions endpoint is how we know to run this command
	AddAggregate(ctx, tsLogger, tsView, rootView.GetUUID(), ch)

	return tsView
}
