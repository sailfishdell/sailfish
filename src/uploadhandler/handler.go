package uploadhandler

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/sailfish/src/eventwaiter"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/event"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func Setup(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus) {
	eh.RegisterCommand(func() eh.Command { return &POST{eventBus: eb} })
	eh.RegisterEventData(GenericUploadEvent, func() eh.EventData { return &GenericUploadEventData{} })
}

const (
	GenericUploadEvent = eh.EventType("GenericUploadEvent")
	POSTCommand        = eh.CommandType("GenericUploadHandler:POST")
	MAX_MEMORY         = 1 * 1024 * 1024 // max memory for the mulitpart form parsing
)

var (
	UploadDirectory string
)

type GenericUploadEventData struct {
	ID          eh.UUID // id of aggregate
	CmdID       eh.UUID
	ResourceURI string

	PostFile  interface{}
	LocalFile interface{}
}

// HTTP POST Command
type POST struct {
	eventBus eh.EventBus

	ID      eh.UUID           `json:"id"`
	CmdID   eh.UUID           `json:"cmdid"`
	Headers map[string]string `eh:"optional"`

	// make sure to make everything else optional or this will fail
	PostFile  interface{} `eh:"optional"`
	LocalFile interface{} `eh:"optional"`
}

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&POST{})

func (c *POST) AggregateType() eh.AggregateType { return domain.AggregateType }
func (c *POST) AggregateID() eh.UUID            { return c.ID }
func (c *POST) CommandType() eh.CommandType     { return POSTCommand }
func (c *POST) SetAggID(id eh.UUID)             { c.ID = id }
func (c *POST) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *POST) ParseHTTPRequest(r *http.Request) error {

	if err := r.ParseMultipartForm(MAX_MEMORY); err != nil {
		return err
	}

	file, handler, err := r.FormFile("upload")
	if err != nil {
		return err
	}
	defer file.Close()

	f, err := ioutil.TempFile(".", UploadDirectory+"/upld")
	if err != nil {
		return err
	}
	defer f.Close()

	io.Copy(f, file)
	c.LocalFile = f.Name()
	c.PostFile = handler.Filename

	return nil
}
func (c *POST) Handle(ctx context.Context, a *domain.RedfishResourceAggregate) error {
	// Upload handler needs to send HTTP response
	c.eventBus.PublishEvent(ctx, eh.NewEvent(GenericUploadEvent, &GenericUploadEventData{
		ID:          c.ID,
		CmdID:       c.CmdID,
		ResourceURI: a.ResourceURI,
		PostFile:    c.PostFile,
		LocalFile:   c.LocalFile,
	}, time.Now()))
	return nil
}

func SelectUpload(uri string) func(eh.Event) bool {
	return func(event eh.Event) bool {
		if event.EventType() != GenericUploadEvent {
			return false
		}
		if data, ok := event.Data().(*GenericUploadEventData); ok {
			if data.ResourceURI == uri {
				return true
			}
		}
		return false
	}
}

type prop interface {
	GetProperty(string) interface{}
}

type handler func(context.Context, eh.Event, *domain.HTTPCmdProcessedData) error

type actionrunner interface {
	GetAction(string) view.Action
}

func CreateViewUpload(
	ctx context.Context,
	logger log.Logger,
	uploadURI string,
	uploadDIR string,
	timeout int,
	vw actionrunner,
	ch eh.CommandHandler,
	eb eh.EventBus,
) {
	logger.Info("CREATING UPLOAD", "uploadURI", uploadURI)

	if uploadDIR != "" {
		UploadDirectory = uploadDIR
	} else {
		UploadDirectory = "."
	}

	logger.Crit("upload directory", "UploadDirectory", UploadDirectory)

	EventPublisher := eventpublisher.NewEventPublisher()
	// TODO: fix MatchAny
	eb.AddHandler(eh.MatchEvent(domain.HTTPCmdProcessed), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Upload Event Timeout Publisher"))
	EventPublisher.AddObserver(EventWaiter)

	// The following redfish resource is created only for the purpose of being
	// a 'receiver' for the upload command specified above.
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			ResourceURI: uploadURI,
			Type:        "Upload",
			Context:     "Upload",
			Plugin:      "GenericUploadHandler",
			Privileges: map[string]interface{}{
				"POST": []string{"ConfigureManager"},
			},
			Properties: map[string]interface{}{},
		},
	)

	// stream processor for upload events
	sp, err := event.NewESP(ctx, event.CustomFilter(SelectUpload(uploadURI)), event.SetListenerName("uploadhandler"))
	if err != nil {
		logger.Error("Failed to create event stream processor", "err", err)
		return
	}
	go sp.RunForever(func(event eh.Event) {

		ourCmdID := event.Data().(*GenericUploadEventData).CmdID
		ourLocalFile := event.Data().(*GenericUploadEventData).LocalFile.(string)

		logger.Debug("Upload running!", "uploadURI", uploadURI)

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

		// unable to create the listiner, thats bad...
		if err != nil {
			return
		}

		go func() {
			defer os.Remove(ourLocalFile)
			defer listener.Close()
			inbox := listener.Inbox()
			timer := time.NewTimer(time.Duration(timeout) * time.Second)
			for {
				select {
				case <-inbox:
					// got an event from the pump with our exact cmdid, we are done
					return

				case <-timer.C:
					eventData := &domain.HTTPCmdProcessedData{
						CommandID:  ourCmdID,
						Results:    map[string]interface{}{"msg": "Timed Out!"},
						StatusCode: 500,
						Headers:    map[string]string{},
					}
					responseEvent := eh.NewEvent(domain.HTTPCmdProcessed, eventData, time.Now())
					eb.PublishEvent(ctx, responseEvent)
					logger.Error("Upload timeout", "ID", ourCmdID)

				// user cancelled curl request before we could get a response
				case <-ctx.Done():
					return
				}
			}
		}()

	})
}

func WithUpload(ctx context.Context, logger log.Logger, uriSuffix string, dir string, timeout int, ch eh.CommandHandler, eb eh.EventBus) view.Option {
	return func(s *view.View) error {
		uri := s.GetURIUnlocked() + uriSuffix
		CreateViewUpload(ctx, logger, uri, dir, timeout, s, ch, eb)
		return nil
	}
}
