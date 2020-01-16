package uploadhandler

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
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

func debugError(r *http.Request) {
	for name, headers := range r.Header {
		for _, h := range headers {
			fmt.Printf("\n%v : %v\n", name, h)
		}
	}
}

func octetStreamUploadHandler(c *POST, r *http.Request) error {
	var localFile string

	// no file specified so just use the upload name so the
	// action needs to be based on the URL.
	uploadFile := "octet_stream.file"

	// prepare the destination file (tmpfile name)
	dst, err := ioutil.TempFile(".", UploadDir+"/upld")
	if err != nil {
		fmt.Printf("\nunable to create upload file, %s\n", err)
		return err
	}
	defer dst.Close()

	localFile = dst.Name()
	c.Files[uploadFile] = localFile

	n, err := io.Copy(dst, r.Body)
	if err != nil {
		fmt.Printf("\ncopy failed, %s\n", err)
		defer os.Remove(localFile)
		fmt.Printf("remove %s\n", localFile)
		return err
	}

	fmt.Printf("%d bytes are recieved.\n", n)
	fmt.Printf("\nupload %d %s to %s\n", n, uploadFile, localFile)

	return nil
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
		fmt.Printf("\nno reader %s\n", err)
		httputil.DumpRequest(r, false)
		// sailfish logs go to EMMC.  oauth token can not be printed there
		// debugError(r)
		return octetStreamUploadHandler(c, r)
		//return err
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
		if err != nil {
			fmt.Printf("\nunable to create upload file, %s\n", err)
			return err
		}
		defer dst.Close()
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
			fmt.Printf("\ncopy failed, %s\n", err)
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

type BusObjs interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
	GetCommandHandler() eh.CommandHandler
}

func StartService(ctx context.Context, logger log.Logger, d BusObjs) *Service {
	ret := &Service{
		ch:      d.GetCommandHandler(),
		eb:      d.GetBus(),
		uploads: map[string]*registration{},
	}

	listener := eventwaiter.NewListener(ctx, logger, d.GetWaiter(), func(ev eh.Event) bool {
		return ev.EventType() == GenericUploadEvent
	})
	// never calling listener.Close() because we can't shut this down

	go listener.ProcessEvents(ctx, func(event eh.Event) {
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
			defer ret.RUnlock()
			logger.Crit("URI", "URI", data.ResourceURI, "pending", ret.uploads)
			reg, ok := ret.uploads[data.ResourceURI]
			if !ok {
				// didn't find upload for this URL
				logger.Crit("COULD NOT FIND URI", "URI", data.ResourceURI)
				return
			}
			handler = reg.view.GetUpload(reg.uploadName)
			logger.Crit("URI", "uri", data.ResourceURI)

		}

		// defer removing the uploaded file
		//  	for _, lf := range event.Data().(*GenericUploadEventData).Files {
		//  		defer os.Remove(lf)
		//  		logger.Crit("remove upload file", "FILE", lf)
		//  	}

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
			go ret.eb.PublishEvent(ctx, responseEvent)
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

		// only create receiver aggregate for the PATCH for cases where it is a different
		// UIR from the view (this lets us handle PATCH on the actual view)
		// TODO: FIX THIS CORRECTLY, without the check for FirmwareInventory GET requests there fail
		if uriSuffix != "/FirmwareInventory" && v.GetURIUnlocked() != uri {
			// The following redfish resource is created only for the purpose of being
			// a 'receiver' for the action command specified above.
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
		}

		return nil
	}
}
