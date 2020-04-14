package metric

import (
	"context"
	"fmt"
	"io"
	"time"

	eh "github.com/looplab/eventhorizon"
)

type Commander interface {
	GetRequestID() eh.UUID
	ResponseWaitFn() func(eh.Event) bool
	SetContext(context.Context)
	SetResponseHandlers(headerH headerHandler, statusH statusHandler, writeH io.Writer)
}

func CommandFactory(et eh.EventType) func() (Commander, eh.Event, error) {
	return func() (Commander, eh.Event, error) {
		data, err := eh.CreateEventData(et)
		if err != nil {
			return nil, nil, fmt.Errorf("could not create request report command: %w", err)
		}
		return data.(Commander), eh.NewEvent(et, data, time.Now()), nil
	}
}

type statusHandler func(int)
type headerHandler func(string, string)

type Command struct {
	RequestID             eh.UUID
	ResponseType          eh.EventType
	requestCtx            context.Context
	responseStatusHandler statusHandler
	responseHeaderHandler headerHandler
	responseWriteHandler  io.Writer
}

func NewCommand(t eh.EventType) Command {
	return Command{RequestID: eh.NewUUID(), ResponseType: t}
}

type getCRer interface {
	getCR(context.Context, eh.UUID) *CommandResponse
}

func (cmd *Command) SetContext(ctx context.Context) {
	cmd.requestCtx = ctx
}

func (cmd *Command) NewStreamingResponse() (eh.Event, error) {
	data, err := eh.CreateEventData(cmd.ResponseType)
	if err != nil {
		return nil, fmt.Errorf("could not create response: %w", err)
	}
	getcr, ok := data.(getCRer)
	if !ok {
		return nil, fmt.Errorf("internal programming error: response encoded in cmd wasn't a response type: %T -> %+v", data, data)
	}

	cr := getcr.getCR(cmd.requestCtx, cmd.RequestID)
	cr.responseStatusHandler = cmd.responseStatusHandler
	cr.responseHeaderHandler = cmd.responseHeaderHandler
	cr.responseWriteHandler = cmd.responseWriteHandler

	return eh.NewEvent(cmd.ResponseType, data, time.Now()), nil
}

func (cmd *Command) NewResponseEvent(responseErr error) (eh.Event, error) {
	data, err := eh.CreateEventData(cmd.ResponseType)
	if err != nil {
		return nil, fmt.Errorf("could not create response: %w", err)
	}
	getcr, ok := data.(getCRer)
	if !ok {
		return nil, fmt.Errorf("internal programming error: response encoded in cmd wasn't a response type: %T -> %+v", data, data)
	}

	cr := getcr.getCR(cmd.requestCtx, cmd.RequestID)
	cr.responseStatusHandler = cmd.responseStatusHandler
	cr.responseHeaderHandler = cmd.responseHeaderHandler
	cr.responseWriteHandler = cmd.responseWriteHandler

	status := HTTPStatusOk
	out := []byte("SUCCESS\n")

	if responseErr != nil {
		status = HTTPStatusBadRequest
		out = []byte("FAILED\n")
	}

	cr.WriteDefaultHeaders()
	cr.WriteStatus(status)
	cr.Write(out)

	return eh.NewEvent(cmd.ResponseType, cr, time.Now()), nil
}

func (cmd *Command) ResponseWaitFn() func(eh.Event) bool {
	return func(evt eh.Event) bool {
		if evt.EventType() != cmd.ResponseType {
			return false
		}
		if data, ok := evt.Data().(Responser); ok && data.matchRequestID(cmd.RequestID) {
			return true
		}
		return false
	}
}

func (cmd *Command) GetRequestID() eh.UUID {
	return cmd.RequestID
}

func (cmd *Command) SetResponseHandlers(headerH headerHandler, statusH statusHandler, writeH io.Writer) {
	cmd.responseHeaderHandler = headerH
	cmd.responseStatusHandler = statusH
	cmd.responseWriteHandler = writeH
}

type Responser interface {
	matchRequestID(eh.UUID) bool
}

type CommandResponse struct {
	RequestID  eh.UUID
	requestCtx context.Context

	responseStatusHandler statusHandler
	responseHeaderHandler headerHandler
	responseWriteHandler  io.Writer
}

func (cr *CommandResponse) getCR(ctx context.Context, rID eh.UUID) *CommandResponse {
	cr.RequestID = rID
	cr.requestCtx = ctx
	return cr
}

func (cr *CommandResponse) matchRequestID(id eh.UUID) bool {
	return cr.RequestID == id
}

// ============================================================================
// command handler API - the command handler for commands should create the
// response event, then use these response apis. When completely done handling
// request, publish event.
//
// 		WriteHeader
// 		WriteStatus
// 		Write
// ============================================================================

func (cr *CommandResponse) WriteHeader(header string, value string) {
	if cr.responseHeaderHandler != nil {
		cr.responseHeaderHandler(header, value)
	}
}

// ONLY CALL ONCE
func (cr *CommandResponse) WriteStatus(s int) {
	if cr.responseStatusHandler != nil {
		cr.responseStatusHandler(s)
	}
}

func (cr *CommandResponse) Write(data []byte) (int, error) {
	if cr.responseWriteHandler != nil {
		return cr.responseWriteHandler.Write(data)
	}
	return 0, nil
}

func (cr *CommandResponse) WriteDefaultHeaders() {
	// common headers
	cr.WriteHeader("OData-Version", "4.0")
	cr.WriteHeader("Server", "metric-engine")
	cr.WriteHeader("Content-Type", "application/json; charset=utf-8")
	cr.WriteHeader("Connection", "keep-alive")
	cr.WriteHeader("Cache-Control", "no-Store,no-Cache")
	cr.WriteHeader("Pragma", "no-cache")

	// security headers
	cr.WriteHeader("Strict-Transport-Security", "max-age=63072000; includeSubDomains") // for A+ SSL Labs score
	cr.WriteHeader("Access-Control-Allow-Origin", "*")
	cr.WriteHeader("X-Frame-Options", "DENY")
	cr.WriteHeader("X-XSS-Protection", "1; mode=block")
	cr.WriteHeader("X-Content-Security-Policy", "default-src 'self'")
	cr.WriteHeader("X-Content-Type-Options", "nosniff")

	// compatibility headers
	cr.WriteHeader("X-UA-Compatible", "IE=11")
}
