package domain

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/xerrors" // official backport of GO 1.13 new errors impl

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/looplab/aggregatestore"
)

type CommandHandler struct {
	t     eh.AggregateType
	store aggregatestore.AggregateStore // commands to manage stored aggregates
}

// NewCommandHandler creates a new CommandHandler for an aggregate type.
func NewCommandHandler(t eh.AggregateType, store aggregatestore.AggregateStore) (*CommandHandler, error) {
	h := &CommandHandler{
		t:     eh.AggregateType("RedfishResource"),
		store: store,
	}
	return h, nil
}

type ShouldSaver interface {
	ShouldSave() bool
}

type Versioner interface {
	GetVersion() int
}

func GetAggregateVersion(agg eh.Aggregate) int {
	if v, ok := agg.(Versioner); ok {
		return v.GetVersion()
	}
	return 0
}

var ErrWrongVersion error = errors.New("wrong version")

func (h *CommandHandler) HandleCommand(ctx context.Context, cmd eh.Command) error {
	err := eh.CheckCommand(cmd)
	if err != nil {
		return err
	}

	for {
		a, err := h.store.Load(ctx, h.t, cmd.AggregateID())
		if err != nil {
			return err
		}
		if a == nil {
			return eh.ErrAggregateNotFound
		}

		err = a.HandleCommand(ctx, cmd)
		if err != nil {
			return err
		}

		// Check the ShouldSaver interface: if it doesn't have a saver interface or
		// the object says not to save, let's not save it!
		if ss, ok := cmd.(ShouldSaver); !ok || !ss.ShouldSave() {
			return nil
		}

		err = h.store.Save(ctx, a)
		if xerrors.Is(err, ErrWrongVersion) {
			continue
		}
		if err != nil {
			fmt.Printf("HELP!\n")
		}
		return nil
	}

	return nil
}
