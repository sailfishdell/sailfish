package redfishserver

import (
	"context"
	"errors"
	eh "github.com/looplab/eventhorizon"
	"net/http"
	"strings"

	"github.com/superchalupa/go-redfish/domain"

	"fmt"
)

var _ = fmt.Printf

type Response struct {
	// status code is for external users
	StatusCode int
	Headers    map[string]string
	Output     interface{}
}

// Service is the business logic for a redfish server
type Service interface {
	GetRedfishResource(ctx context.Context, r *http.Request, privileges []string) (*Response, error)
	RedfishResourceHandler(ctx context.Context, r *http.Request, privileges []string) (*Response, error)
	domain.DDDFunctions
}

// ServiceMiddleware is a chainable behavior modifier for Service.
type ServiceMiddleware func(Service) Service

var (
	// ErrNotFound is returned when a request isnt present (404)
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("Unauthorized") // 401... missing or bad authentication
	ErrForbidden    = errors.New("Forbidden")    // should be 403 (you are authenticated, but dont have permissions to this object)
)

// ServiceConfig is where we store the current service data
type ServiceConfig struct {
	domain.DDDFunctions
	httpsagas *domain.HTTPSagaList
}

// NewService is how we initialize the business logic
func NewService(d domain.DDDFunctions) Service {
	cfg := ServiceConfig{
		httpsagas:    domain.NewHTTPSagaList(d),
		DDDFunctions: d,
	}

	return &cfg
}

func (rh *ServiceConfig) GetRedfishResource(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
	noHashPath := strings.SplitN(r.URL.Path, "#", 2)[0]

	// we have the tree ID, fetch an updated copy of the actual tree
	tree, err := domain.GetTree(ctx, rh.GetReadRepo(), rh.GetTreeID())
	if err != nil {
		return &Response{StatusCode: http.StatusInternalServerError, Output: map[string]interface{}{"error": err.Error()}}, err
	}

	// now that we have the tree, look up the actual URI in that tree to find
	// the object UUID, then pull that from the repo
	requested, err := rh.GetReadRepo().Find(ctx, tree.Tree[noHashPath])
	if err != nil {
		return &Response{StatusCode: http.StatusNotFound, Output: map[string]interface{}{"error": err.Error()}}, nil
	}
	item, ok := requested.(*domain.RedfishResource)
	if !ok {
		return &Response{StatusCode: http.StatusInternalServerError}, errors.New("Expected a RedfishResource, but got something strange.")
	}

	return &Response{StatusCode: http.StatusOK, Output: item.Properties, Headers: item.Headers}, nil
}

func (rh *ServiceConfig) RedfishResourceHandler(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {
	// we shouldn't actually ever get a path with a hash, I don't think.
	noHashPath := strings.SplitN(r.URL.Path, "#", 2)[0]

	// we have the tree ID, fetch an updated copy of the actual tree
	tree, err := domain.GetTree(ctx, rh.GetReadRepo(), rh.GetTreeID())
	if err != nil {
		return &Response{StatusCode: http.StatusInternalServerError}, err
	}

	// now that we have the tree, look up the actual URI in that tree to find
	// the object UUID, then pull that from the repo
	requested, err := rh.GetReadRepo().Find(ctx, tree.Tree[noHashPath])
	if err != nil {
		// it's ok if obj not found
		return &Response{StatusCode: http.StatusNotFound}, nil
	}
	item, ok := requested.(*domain.RedfishResource)
	if !ok {
		return &Response{StatusCode: http.StatusInternalServerError}, errors.New("Expected a RedfishResource, but got something strange.")
	}

	cmdUUID := eh.NewUUID()

	// we send a command and then wait for a completion event. Set up the wait here.
	waitID, resultChan := rh.GetEventWaiter().SetupWait(func(event eh.Event) bool {
		if event.EventType() != domain.HTTPCmdProcessedEvent {
			return false
		}
		if data, ok := event.Data().(*domain.HTTPCmdProcessedData); ok {
			if data.CommandID == cmdUUID {
				return true
			}
		}
		return false
	})

	defer rh.GetEventWaiter().CancelWait(waitID)

	// all of the available operations are registered with httpsagas, find one.
	err = rh.httpsagas.RunHTTPOperation(ctx, rh.GetTreeID(), cmdUUID, item, r)
	if err != nil {
		return &Response{StatusCode: http.StatusMethodNotAllowed, Output: map[string]interface{}{"error": err.Error()}}, nil
	}

	select {
	case event := <-resultChan:
		d := event.Data().(*domain.HTTPCmdProcessedData)
		return &Response{Output: d.Results, StatusCode: d.StatusCode, Headers: d.Headers}, nil
		// This is an example of how we would set up a job if things time out
		//	case <-time.After(1 * time.Second):
		//		// TODO: Here we could easily automatically create a JOB and return that.
		//		return &Response{StatusCode: http.StatusOK, Output: "JOB"}, nil
	case <-ctx.Done():
		// the requestor cancelled the http request to us. We can abandon
		// returning results, but command will still be processed
		return &Response{StatusCode: http.StatusBadRequest}, nil
	}
}
