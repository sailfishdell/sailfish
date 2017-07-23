package domain

import (
	"context"
	"errors"
	eh "github.com/looplab/eventhorizon"
	"net/http"

	"fmt"
)

var _ = fmt.Println

const (
	// EVENTS
	HTTPCmdProcessedEvent eh.EventType = "HTTPCmdProcessed"
)

type HTTPSaga func(context.Context, eh.UUID, eh.UUID, *RedfishResource, *http.Request) error

// can put URI, odata.type, or odata.context as key
type HTTPSagaList struct {
	sagaList map[string]HTTPSaga
	DDDFunctions
}

type httpsagasetup func(SagaRegisterer)

var Httpsagas []httpsagasetup

func NewHTTPSagaList(d DDDFunctions) *HTTPSagaList {
	sl := HTTPSagaList{
		sagaList:     map[string]HTTPSaga{},
		DDDFunctions: d,
	}

	for _, s := range Httpsagas {
		s(&sl)
	}
	return &sl
}

type SagaRegisterer interface {
	RegisterNewHandler(match string, f HTTPSaga)
	DDDFunctions
}

func (l *HTTPSagaList) RegisterNewHandler(match string, f HTTPSaga) {
	l.sagaList[match] = f
}

type HTTPCmdProcessedData struct {
	CommandID  eh.UUID
	Results    map[string]interface{}
	StatusCode int
	Headers    map[string]string
}

func SetupHTTP(DDDFunctions) {
	// EVENT registration
	eh.RegisterEventData(HTTPCmdProcessedEvent, func() eh.EventData { return &HTTPCmdProcessedData{} })
}

// RunHTTPOperation will try to find a command for an http operation
// Search path:
//      ${HTTP_METHOD}:${odata.id}
//      ${HTTP_METHOD}:${odata.type}
//      ${HTTP_METHOD}:${odata.context}
func (l *HTTPSagaList) RunHTTPOperation(ctx context.Context, treeID, cmdID eh.UUID, resource *RedfishResource, r *http.Request) error {
	aggregateID := resource.Properties["@odata.id"].(string)
	typ := resource.Properties["@odata.type"].(string)
	context := resource.Properties["@odata.context"].(string)
	method := r.Method

	search := []string{
		method + ":" + aggregateID,
		method + ":" + typ,
		method + ":" + context,
		"HandleHTTP",
	}

	for _, s := range search {
		if f, ok := l.sagaList[s]; ok {
			return f(ctx, treeID, cmdID, resource, r)
		}
	}

	return errors.New("Command not found")
}
