package domain

import (
	"context"
	"errors"
	eh "github.com/superchalupa/eventhorizon"
)

type HTTPCmdHandler struct {
}

func NewHTTPCmdHandler(repo eh.ReadWriteRepo, treeID eh.UUID) *HTTPCmdHandler {
	return &HTTPCmdHandler{}
}

// HandlerType implements the HandlerType method of the EventHandler interface.
func (p *HTTPCmdHandler) HandlerType() eh.EventHandlerType {
	return eh.EventHandlerType("HTTPCmdHandler")
}

func (t *HTTPCmdHandler) HandleEvent(ctx context.Context, event eh.Event) error {
	switch event.EventType() {
	}
	return errors.New("Didn't handle anything")
}
