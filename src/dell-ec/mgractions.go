package dell_ec

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
	"github.com/superchalupa/sailfish/src/uploadhandler"
)

type Service struct {
	sync.RWMutex
	outstandingCommands map[eh.UUID]time.Time
	eb                  eh.EventBus
	timer               *time.Timer
	logger              log.Logger
}

type syncEvent interface {
	Done()
}

func NewPumpActionSvc(ctx context.Context, logger log.Logger, eb eh.EventBus) *Service {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchEvent(domain.HTTPCmdProcessed), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Action Timeout Publisher"), eventwaiter.NoAutoRun)
	EventPublisher.AddObserver(EventWaiter)
	go EventWaiter.Run()

	ret := &Service{
		outstandingCommands: map[eh.UUID]time.Time{},
		eb:                  eb,
		timer:               time.NewTimer(time.Duration(1) * time.Hour),
		logger:              logger,
	}

	listener, err := EventWaiter.Listen(ctx, func(event eh.Event) bool {
		if event.EventType() == domain.HTTPCmdProcessed {
			return true
		}
		return false
	})
	if err != nil {
		return nil
	}

	go func() {
		defer listener.Close()
		for {
			select {
			case event := <-listener.Inbox():
				if e, ok := event.(syncEvent); ok {
					e.Done()
				}

				// check if its in our list
				data, ok := event.Data().(*domain.HTTPCmdProcessedData)
				if !ok {
					continue
				}
				cid := data.CommandID
				func() {
					ret.Lock()
					defer ret.Unlock()
					_, ok = ret.outstandingCommands[cid]
					if !ok {
						return
					}
					logger.Info("Got response from pump for action before timeout, cancelling")
					delete(ret.outstandingCommands, cid)
					ret.calculateNewTimer(1 * time.Hour)
				}()

			case <-ret.timer.C:
				// scan our list and see what is timed out
				func() {
					n := time.Now()

					ret.Lock()
					defer ret.Unlock()

					for k, v := range ret.outstandingCommands {
						if n.After(v) {
							ret.PublishTimeout(k)
							delete(ret.outstandingCommands, k)
						}
					}
					ret.calculateNewTimer(1 * time.Hour)
				}()

			// user cancelled curl request before we could get a response
			case <-ctx.Done():
				return
			}
		}
	}()

	return ret
}

func (s *Service) PublishTimeout(cmdid eh.UUID) {
	eventData := &domain.HTTPCmdProcessedData{
		CommandID:  cmdid,
		Results:    map[string]interface{}{"msg": "Timed Out!"},
		StatusCode: 500,
		Headers:    map[string]string{},
	}
	responseEvent := eh.NewEvent(domain.HTTPCmdProcessed, eventData, time.Now())
	s.eb.PublishEvent(context.Background(), responseEvent)
}

func (s *Service) calculateNewTimer(shortest time.Duration) {
	s.logger.Info("Calculating new timeout for action handler", "requested shortest", shortest)
	// set new timer based on whatever is the shortest time coming up
	for _, v := range s.outstandingCommands {
		if time.Until(v) < shortest {
			shortest = time.Until(v)
		}
	}

	if !s.timer.Stop() {
		select {
		case <-s.timer.C:
		default:
		}
	}

	s.logger.Info("Calculating new timeout for action handler", "shortest", shortest)
	if shortest != 0 {
		s.timer.Reset(shortest)
	}
}

func (s *Service) NewPumpAction(maxtimeout int) func(context.Context, eh.Event, *domain.HTTPCmdProcessedData) error {
	return func(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
		s.logger.Info("Running new pump action")
		s.Lock()
		defer s.Unlock()

		// let the upper layer know we have it under control
		retData.StatusCode = 0

		// add to ourstanding command list
		s.outstandingCommands[retData.CommandID] = time.Now().Add(time.Duration(maxtimeout) * time.Second)

		// set timer to whatever is the shortest
		s.calculateNewTimer(time.Duration(maxtimeout) * time.Second)

		return nil
	}
}

func makePumpHandledUpload(name string, maxtimeout int, eb eh.EventBus) func(context.Context, eh.Event, *domain.HTTPCmdProcessedData) error {
	EventPublisher := eventpublisher.NewEventPublisher()

	eb.AddHandler(eh.MatchEvent(domain.HTTPCmdProcessed), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Upload Timeout Publisher"), eventwaiter.NoAutoRun)
	EventPublisher.AddObserver(EventWaiter)
	go EventWaiter.Run()

	return func(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
		// The actionhandler will discard the message if we set statuscode to 0. Client should never see it, and pump can send its own return
		retData.StatusCode = 0
		ourCmdID := retData.CommandID
		fmt.Printf("\n\nGot an action that we expect PUMP to handle. We'll set up a timeout to make sure that happens: %s.\n\n", ourCmdID)

		listener, err := EventWaiter.Listen(ctx, func(event eh.Event) bool {
			if event.EventType() != domain.HTTPCmdProcessed {
				return false
			}
			data, ok := event.Data().(*domain.HTTPCmdProcessedData)
			if ok && data.CommandID == ourCmdID {
				return true
			}
			return false
		})
		if err != nil {
			return err
		}
		listener.Name = "Pump handled action listener: " + name

		ourLocalFiles := event.Data().(*uploadhandler.GenericUploadEventData).Files

		go func() {
			for key, localFile := range ourLocalFiles {
				defer os.Remove(localFile)
				fmt.Printf("\nremove f:%s l:%s on exit\n", key, localFile)
			}
			defer listener.Close()
			timer := time.NewTimer(time.Duration(maxtimeout) * time.Second)
			for {
				select {
				case <-listener.Inbox():
					if e, ok := event.(syncEvent); ok {
						e.Done()
					}

					// got an event from the pump with our exact cmdid, we are done
					return

				case <-timer.C:
					eventData := &domain.HTTPCmdProcessedData{
						CommandID:  event.Data().(*uploadhandler.GenericUploadEventData).CmdID,
						Results:    map[string]interface{}{"msg": "Timed Out!"},
						StatusCode: 500,
						Headers:    map[string]string{},
					}
					responseEvent := eh.NewEvent(domain.HTTPCmdProcessed, eventData, time.Now())
					eb.PublishEvent(ctx, responseEvent)

				// user cancelled curl request before we could get a response
				case <-ctx.Done():
					return
				}
			}
		}()

		return nil
	}
}
