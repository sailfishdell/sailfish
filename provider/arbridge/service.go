package arbridge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/domain"

	"fmt"
)

var _ = fmt.Printf

// Service is the business logic for a redfish server
type Service interface {
	ResourceHandler(ctx context.Context, r *http.Request, privileges []string) (*Response, error)
	domain.DDDFunctions
}

type Response struct {
	// status code is for external users
	StatusCode int
	Headers    map[string]string
	Output     interface{}
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
}

// NewService is how we initialize the business logic
func NewService(d domain.DDDFunctions) Service {
	cfg := ServiceConfig{
		DDDFunctions: d,
	}

	return &cfg
}

type AREventData struct {
	Name  string
	Value string
}

func (rh *ServiceConfig) ResourceHandler(ctx context.Context, r *http.Request, privileges []string) (*Response, error) {

	decoder := json.NewDecoder(r.Body)
	var ev AREventData
	err := decoder.Decode(&ev)

	if err != nil {
		return &Response{StatusCode: 500, Headers: map[string]string{}, Output: nil}, err
	}

	rh.GetEventHandler().HandleEvent(ctx, eh.NewEvent("AREvent", &ev))

	return &Response{
			StatusCode: 200,
			Headers:    map[string]string{},
			Output:     "all happy",
		},
		nil
}
