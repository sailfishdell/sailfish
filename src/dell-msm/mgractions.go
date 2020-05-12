package dell_msm

import (
	"context"
	"time"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

type Service struct {
	outstandingCommands map[eh.UUID]time.Time
	eb                  eh.EventBus
	logger              log.Logger
}

const update = eh.EventType("pump-service-internal")

type UpdateData struct {
	CommandID eh.UUID
	Timeout   time.Time
}
type Timeout struct{}

func NewPumpActionSvc(ctx context.Context, logger log.Logger, d *domain.DomainObjects) *Service {
	ret := &Service{
		outstandingCommands: map[eh.UUID]time.Time{},
		eb:                  d.GetBus(),
		logger:              logger,
	}

	// This timer will publish a timeout message that will drive the lookup in the listener
	timer := time.AfterFunc(time.Duration(10)*time.Second, func() {
		ret.eb.PublishEvent(context.Background(), eh.NewEvent(update, &Timeout{}, time.Now()))
	})

	// This listener runs forever, no need to .Close()
	go eventwaiter.NewListener(ctx, logger, d.GetWaiter(), func(event eh.Event) bool {
		typ := event.EventType()
		return typ == update || typ == domain.HTTPCmdProcessed
	}).ProcessEvents(ctx, func(event eh.Event) {
		switch data := event.Data().(type) {
		case *domain.HTTPCmdProcessedData:
			if _, ok := ret.outstandingCommands[data.CommandID]; !ok {
				return
			}
			delete(ret.outstandingCommands, data.CommandID)
			ret.updateTimer(timer, 1*time.Hour)

		case *UpdateData:
			ret.outstandingCommands[data.CommandID] = data.Timeout
			ret.updateTimer(timer, 1*time.Hour)

		case *Timeout:
			n := time.Now()
			for k, v := range ret.outstandingCommands {
				if n.After(v) {
					ret.PublishTimeout(k)
					delete(ret.outstandingCommands, k)
				}
			}
			ret.updateTimer(timer, 1*time.Hour)
		}
	})

	return ret
}

func (s *Service) PublishTimeout(cmdid eh.UUID) {
	s.eb.PublishEvent(context.Background(), eh.NewEvent(domain.HTTPCmdProcessed,
		&domain.HTTPCmdProcessedData{
			CommandID:  cmdid,
			Results:    map[string]interface{}{"msg": "Timed Out!"},
			StatusCode: 500,
			Headers:    map[string]string{},
		},
		time.Now()))
}

// updateTimer: ONLY CALL FROM listner.ProcessEvents() or you will race
func (s *Service) updateTimer(timer *time.Timer, shortest time.Duration) {
	defer s.logger.Info("Calculated new timeout for action handler", "next_timeout", shortest)
	// stop current timer
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}

	// Calculate the next timeout
	for _, v := range s.outstandingCommands {
		if time.Until(v) < shortest {
			shortest = time.Until(v)
		}
	}

	// Reset the timer
	if shortest > 0 {
		timer.Reset(shortest)
	}
}

func (s *Service) NewPumpAction(maxtimeout int) func(context.Context, eh.Event, *domain.HTTPCmdProcessedData) error {
	return func(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
		s.logger.Info("Running new pump action")

		// let the upper layer know we have it under control
		retData.StatusCode = 0

		// add to ourstanding command list
		s.eb.PublishEvent(context.Background(), eh.NewEvent(update,
			&UpdateData{
				CommandID: retData.CommandID,
				Timeout:   time.Now().Add(time.Duration(maxtimeout) * time.Second)},
			time.Now()))
		return nil
	}
}
