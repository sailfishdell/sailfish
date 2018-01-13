package domain

import (
	"context"
	"encoding/json"
	"fmt"
	eh "github.com/looplab/eventhorizon"
	"net/http"
	"time"
)

func init() {
	eh.RegisterCommand(func() eh.Command { return &GET{} })
	eh.RegisterCommand(func() eh.Command { return &PUT{} })
	eh.RegisterCommand(func() eh.Command { return &PATCH{} })
	eh.RegisterCommand(func() eh.Command { return &POST{} })
	eh.RegisterCommand(func() eh.Command { return &DELETE{} })
	eh.RegisterCommand(func() eh.Command { return &HEAD{} })
	eh.RegisterCommand(func() eh.Command { return &OPTIONS{} })
}

const (
	GETCommand     = eh.CommandType("RedfishResource:GET")
	PUTCommand     = eh.CommandType("RedfishResource:PUT")
	PATCHCommand   = eh.CommandType("RedfishResource:PATCH")
	POSTCommand    = eh.CommandType("RedfishResource:POST")
	DELETECommand  = eh.CommandType("RedfishResource:DELETE")
	HEADCommand    = eh.CommandType("RedfishResource:HEAD")
	OPTIONSCommand = eh.CommandType("RedfishResource:OPTIONS")
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&GET{})
var _ = eh.Command(&PUT{})
var _ = eh.Command(&PATCH{})
var _ = eh.Command(&POST{})
var _ = eh.Command(&DELETE{})
var _ = eh.Command(&HEAD{})
var _ = eh.Command(&OPTIONS{})

// HTTP GET Command
type GET struct {
	ID    eh.UUID `json:"id"`
	CmdID eh.UUID `json:"cmdid"`
}

func (c *GET) AggregateType() eh.AggregateType { return AggregateType }
func (c *GET) AggregateID() eh.UUID            { return c.ID }
func (c *GET) CommandType() eh.CommandType     { return GETCommand }
func (c *GET) SetAggID(id eh.UUID)             { c.ID = id }
func (c *GET) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *GET) SetBody(r *http.Request)         {}
func (c *GET) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	fmt.Printf("HANDLE!\n")
	data := &HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{},
		StatusCode: 200,
		Headers:    map[string]string{"testheader": "foo"},
	}
	for k, v := range a.Properties {
		data.Results[k] = v
	}
	a.eventBus.HandleEvent(ctx, eh.NewEvent(HTTPCmdProcessed, data, time.Now()))
	return nil
}

// HTTP PUT Command
type PUT struct {
	ID    eh.UUID `json:"id"`
	CmdID eh.UUID `json:"cmdid"`
	Body  map[string]interface{}
}

func (c *PUT) AggregateType() eh.AggregateType { return AggregateType }
func (c *PUT) AggregateID() eh.UUID            { return c.ID }
func (c *PUT) CommandType() eh.CommandType     { return PUTCommand }
func (c *PUT) SetAggID(id eh.UUID)             { c.ID = id }
func (c *PUT) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *PUT) SetBody(r *http.Request) {
	json.NewDecoder(r.Body).Decode(&c.Body)
}
func (c *PUT) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	fmt.Printf("HANDLE!\n")
	a.eventBus.HandleEvent(ctx, eh.NewEvent(HTTPCmdProcessed, &HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{"FOO": "BAR"},
		StatusCode: 200,
		Headers:    map[string]string{"testheader": "foo"},
	}, time.Now()))
	return nil
}

// HTTP PATCH Command
type PATCH struct {
	ID    eh.UUID `json:"id"`
	CmdID eh.UUID `json:"cmdid"`
	Body  map[string]interface{}
}

func (c *PATCH) AggregateType() eh.AggregateType { return AggregateType }
func (c *PATCH) AggregateID() eh.UUID            { return c.ID }
func (c *PATCH) CommandType() eh.CommandType     { return PATCHCommand }
func (c *PATCH) SetAggID(id eh.UUID)             { c.ID = id }
func (c *PATCH) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *PATCH) SetBody(r *http.Request) {
	json.NewDecoder(r.Body).Decode(&c.Body)
}
func (c *PATCH) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	fmt.Printf("HANDLE!\n")
	a.eventBus.HandleEvent(ctx, eh.NewEvent(HTTPCmdProcessed, &HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{"FOO": "BAR"},
		StatusCode: 200,
		Headers:    map[string]string{"testheader": "foo"},
	}, time.Now()))
	return nil
}

// HTTP POST Command
type POST struct {
	ID    eh.UUID `json:"id"`
	CmdID eh.UUID `json:"cmdid"`
	Body  map[string]interface{}
}

func (c *POST) AggregateType() eh.AggregateType { return AggregateType }
func (c *POST) AggregateID() eh.UUID            { return c.ID }
func (c *POST) CommandType() eh.CommandType     { return POSTCommand }
func (c *POST) SetAggID(id eh.UUID)             { c.ID = id }
func (c *POST) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *POST) SetBody(r *http.Request) {
	json.NewDecoder(r.Body).Decode(&c.Body)
}
func (c *POST) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	fmt.Printf("HANDLE!\n")
	a.eventBus.HandleEvent(ctx, eh.NewEvent(HTTPCmdProcessed, &HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{"FOO": "BAR"},
		StatusCode: 200,
		Headers:    map[string]string{"testheader": "foo"},
	}, time.Now()))
	return nil
}

// HTTP DELETE Command
type DELETE struct {
	ID    eh.UUID `json:"id"`
	CmdID eh.UUID `json:"cmdid"`
}

func (c *DELETE) AggregateType() eh.AggregateType { return AggregateType }
func (c *DELETE) AggregateID() eh.UUID            { return c.ID }
func (c *DELETE) CommandType() eh.CommandType     { return DELETECommand }
func (c *DELETE) SetAggID(id eh.UUID)             { c.ID = id }
func (c *DELETE) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *DELETE) SetBody(r *http.Request)         {}
func (c *DELETE) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	fmt.Printf("HANDLE!\n")
	a.eventBus.HandleEvent(ctx, eh.NewEvent(HTTPCmdProcessed, &HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{"FOO": "BAR"},
		StatusCode: 200,
		Headers:    map[string]string{"testheader": "foo"},
	}, time.Now()))
	return nil
}

// HTTP HEAD Command
type HEAD struct {
	ID    eh.UUID `json:"id"`
	CmdID eh.UUID `json:"cmdid"`
}

func (c *HEAD) AggregateType() eh.AggregateType { return AggregateType }
func (c *HEAD) AggregateID() eh.UUID            { return c.ID }
func (c *HEAD) CommandType() eh.CommandType     { return HEADCommand }
func (c *HEAD) SetAggID(id eh.UUID)             { c.ID = id }
func (c *HEAD) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *HEAD) SetBody(r *http.Request)         {}
func (c *HEAD) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	fmt.Printf("HANDLE!\n")
	a.eventBus.HandleEvent(ctx, eh.NewEvent(HTTPCmdProcessed, &HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{"FOO": "BAR"},
		StatusCode: 200,
		Headers:    map[string]string{"testheader": "foo"},
	}, time.Now()))
	return nil
}

// HTTP OPTIONS Command
type OPTIONS struct {
	ID    eh.UUID `json:"id"`
	CmdID eh.UUID `json:"cmdid"`
}

func (c *OPTIONS) AggregateType() eh.AggregateType { return AggregateType }
func (c *OPTIONS) AggregateID() eh.UUID            { return c.ID }
func (c *OPTIONS) CommandType() eh.CommandType     { return OPTIONSCommand }
func (c *OPTIONS) SetAggID(id eh.UUID)             { c.ID = id }
func (c *OPTIONS) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *OPTIONS) SetBody(r *http.Request)         {}
func (c *OPTIONS) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	fmt.Printf("HANDLE!\n")
	a.eventBus.HandleEvent(ctx, eh.NewEvent(HTTPCmdProcessed, &HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{"FOO": "BAR"},
		StatusCode: 200,
		Headers:    map[string]string{"testheader": "foo"},
	}, time.Now()))
	return nil
}
