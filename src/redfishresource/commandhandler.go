package domain

import (
	"context"
	"errors"
	"golang.org/x/xerrors" // official backport of GO 1.13 new errors impl

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/looplab/aggregatestore"
)

type CommandHandler struct {
	t     eh.AggregateType
	store aggregatestore.AggregateStore
	bus   eh.EventBus
}

// NewCommandHandler creates a new CommandHandler for an aggregate type.
func NewCommandHandler(t eh.AggregateType, store aggregatestore.AggregateStore, bus eh.EventBus) (*CommandHandler, error) {
	h := &CommandHandler{
		t:     eh.AggregateType("RedfishResource"),
		store: store,
		bus:   bus,
	}
	return h, nil
}

type ShouldSaver interface {
	ShouldSave() bool
}

type Versioner interface {
	GetVersion() int
}

var ErrWrongVersion error = errors.New("wrong version")

func (h *CommandHandler) HandleCommand(ctx context.Context, cmd eh.Command) error {
	err := eh.CheckCommand(cmd)
	if err != nil {
		return err
	}

	a, err := h.store.Load(ctx, h.t, cmd.AggregateID())
	if err != nil {
		return xerrors.Errorf("Error trying to load aggregate: %w", err)
	}
	if a == nil {
		return eh.ErrAggregateNotFound
	}

	err = a.HandleCommand(ctx, cmd)
	if err != nil {
		return xerrors.Errorf("Error handling the command: %w", err)
	}

	type EventPublisher interface {
		EventsToPublish() []eh.Event
		ClearEvents()
	}

	var events []eh.Event
	publisher, ok := a.(EventPublisher)
	if ok && h.bus != nil {
		events = publisher.EventsToPublish()
		publisher.ClearEvents()
	}

	// Check the ShouldSaver interface: if it doesn't have a saver interface or
	// the object says not to save, let's not save it!
	if ss, ok := cmd.(ShouldSaver); ok && ss.ShouldSave() {
		err = h.store.Save(ctx, a)
		if err != nil {
			return xerrors.Errorf("Error saving the aggregate: %w", err)
		}
	}

	// Publish events if supported by the aggregate.
	for _, e := range events {
		h.bus.PublishEvent(ctx, e)
	}

	return nil
}
