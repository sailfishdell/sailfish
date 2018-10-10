package uploadhandler

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"
	//  eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	//  "github.com/superchalupa/sailfish/src/eventwaiter"
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
	UploadDir          = "perm"
)

type GenericUploadEventData struct {
	ID          eh.UUID // id of aggregate
	CmdID       eh.UUID
	ResourceURI string

	Files map[string]string
}

// HTTP POST Command
type POST struct {
	eventBus eh.EventBus

	ID      eh.UUID           `json:"id"`
	CmdID   eh.UUID           `json:"cmdid"`
	Headers map[string]string `eh:"optional"`

	// make sure to make everything else optional or this will fail
	Files map[string]string `eh:"optional"`
}

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&POST{})

func (c *POST) AggregateType() eh.AggregateType { return domain.AggregateType }
func (c *POST) AggregateID() eh.UUID            { return c.ID }
func (c *POST) CommandType() eh.CommandType     { return POSTCommand }
func (c *POST) SetAggID(id eh.UUID)             { c.ID = id }
func (c *POST) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *POST) ParseHTTPRequest(r *http.Request) error {
	if r.Method != "POST" {
		return nil
	}

	var localFile string
	var uploadFile string
	length := r.ContentLength

	fmt.Printf("\nupload URI %s\n", r.RequestURI)

	// make a map of the uploaded file name to the file it was
	// actually stored as.. file[ec_fwupd.d9] = "tmp12345"
	// this will be sent to the pump as a generic upload event
	// that along with the URL *should* ??? be enough for the pump
	// to determine what to do with the file(s)
	c.Files = make(map[string]string)

	// write the file to a temporary one
	reader, err := r.MultipartReader()
	if err != nil {
		return err
	}
	// copy each part to destination.
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		// if part.FileName() is empty, skip this iteration.
		if part.FileName() == "" {
			continue
		}

		uploadFile = part.FileName()

		// prepare the destination file (tmpfile name)
		dst, err := ioutil.TempFile(".", UploadDir+"/upld")
		defer dst.Close()
		if err != nil {
			return err
		}
		localFile = dst.Name()
		c.Files[uploadFile] = localFile

		// for debug TODO: remove later
		fmt.Printf("\nupload %d %s to %s\n", length, uploadFile, localFile)

		if _, err := io.Copy(dst, part); err != nil {
			// ERROR!! remove any files that may have been
			// partially transfered
			for _, lf := range c.Files {
				defer os.Remove(lf)
				fmt.Printf("remove %s\n", lf)
			}
			return err
		}
	}

	return nil
}
func (c *POST) Handle(ctx context.Context, a *domain.RedfishResourceAggregate) error {
	// Upload handler needs to send HTTP response
	c.eventBus.PublishEvent(ctx, eh.NewEvent(GenericUploadEvent, &GenericUploadEventData{
		ID:          c.ID,
		CmdID:       c.CmdID,
		ResourceURI: a.ResourceURI,
		Files:       c.Files,
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

type uploadrunner interface {
	GetUpload(string) view.Upload
}

type registration struct {
	uploadName string
	view       uploadrunner
}

type Service struct {
	sync.RWMutex
	ch      eh.CommandHandler
	eb      eh.EventBus
	uploads map[string]*registration
}

func StartService(ctx context.Context, logger log.Logger, ch eh.CommandHandler, eb eh.EventBus) *Service {
	ret := &Service{
		ch:      ch,
		eb:      eb,
		uploads: map[string]*registration{},
	}

	// stream processor for upload events
	sp, err := event.NewESP(ctx, event.CustomFilter(func(ev eh.Event) bool {
		if ev.EventType() == GenericUploadEvent {
			return true
		}
		return false
	}), event.SetListenerName("uploadhandler"))
	if err != nil {
		logger.Error("Failed to create event stream processor", "err", err)
		return nil
	}
	go sp.RunForever(func(event eh.Event) {
		eventData := &domain.HTTPCmdProcessedData{
			CommandID:  event.Data().(*GenericUploadEventData).CmdID,
			Results:    map[string]interface{}{"msg": "Not Implemented"},
			StatusCode: 500,
			Headers:    map[string]string{},
		}

		logger.Crit("Upload running!")
		var handler view.Upload
		if data, ok := event.Data().(*GenericUploadEventData); ok {
			ret.RLock()
			reg := ret.uploads[data.ResourceURI]
			handler = reg.view.GetUpload(reg.uploadName)
			logger.Crit("URI", "uri", data.ResourceURI)
			ret.RUnlock()
		}

		logger.Crit("handler", "handler", handler)

		// only send out our pre-canned response if no handler exists (above), or if handler sets the event status code to 0
		// for example, if data pump is going to directly send an httpcmdprocessed.
		if handler != nil {
			handler(ctx, event, eventData)
		} else {
			logger.Warn("UNHANDLED upload event: no function handler set up for this event.", "event", event)
		}
		if eventData.StatusCode != 0 {
			responseEvent := eh.NewEvent(domain.HTTPCmdProcessed, eventData, time.Now())
			go eb.PublishEvent(ctx, responseEvent)
		}
	})

	return ret
}

func (s *Service) WithUpload(ctx context.Context, name string, uriSuffix string, a view.Upload) view.Option {
	return func(v *view.View) error {
		uri := v.GetURIUnlocked() + uriSuffix
		v.SetUploadUnlocked(name, a)
		v.SetUploadURIUnlocked(name, uri)

		s.Lock()
		defer s.Unlock()
		s.uploads[uri] = &registration{
			uploadName: name,
			view:       v,
		}

		// The following redfish resource is created only for the purpose of being
		// a 'receiver' for the upload command specified above.
		s.ch.HandleCommand(
			ctx,
			&domain.CreateRedfishResource{
				ID:          eh.NewUUID(),
				ResourceURI: uri,
				Type:        "Upload",
				Context:     "Upload",
				Plugin:      "GenericUploadHandler",
				Privileges: map[string]interface{}{
					"POST": []string{"ConfigureManager"},
				},
				Properties: map[string]interface{}{},
			},
		)

		return nil
	}
}
