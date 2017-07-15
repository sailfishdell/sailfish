package domain

import (
	"context"
	"log"

	eh "github.com/looplab/eventhorizon"
)

// Logger is a simple event handler for logging all events.
type Logger struct{}

// Notify implements the HandleEvent method of the EventHandler interface.
func (l *Logger) Notify(ctx context.Context, event eh.Event) error {
	log.Println("event:", event)
	return nil
}
