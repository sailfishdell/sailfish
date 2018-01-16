package session

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	XAuthTokenRefreshEvent = eh.EventType("XAuthTokenRefresh")
)

type XAuthTokenRefreshData struct {
	SessionURI string
}

func init() {
	eh.RegisterEventData(XAuthTokenRefreshEvent, func() eh.EventData {
		return &XAuthTokenRefreshData{}
	})
}
