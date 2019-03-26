package stdmeta

import (
	"context"
	"strings"
	"time"

	eh "github.com/looplab/eventhorizon"
	ah "github.com/superchalupa/sailfish/src/actionhandler"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

var (
	ssoInfoPlugin = domain.PluginType("ssoinfo")
)

const (
	RequestGetSsoInfo = eh.EventType("Request:GetSsoInfo")
)

func InitializeSsoinfo(d *domain.DomainObjects) {
	domain.RegisterPlugin(func() domain.Plugin { return &ssoInfo{d: d} })
	eh.RegisterEventData(RequestGetSsoInfo, func() eh.EventData { return &GetSsoInfoData{} })
}

type ssoInfo struct {
	d *domain.DomainObjects
}

type GetSsoInfoData struct {
	ID    eh.UUID // id of aggregate
	IomID string
}

func (t *ssoInfo) PluginType() domain.PluginType { return ssoInfoPlugin }

func (t *ssoInfo) PropertyGet(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	auth *domain.RedfishAuthorizationProperty,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) error {
	reqUUID := eh.NewUUID()
	cmdID := reqUUID

	// to avoid races, set up our listener first
	timeoutctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	l, err := t.d.EventWaiter.Listen(timeoutctx, func(event eh.Event) bool {
		if event.EventType() != domain.HTTPCmdProcessed {
			return false
		}
		if data, ok := event.Data().(*domain.HTTPCmdProcessedData); ok {
			if data.CommandID == cmdID {
				rrp.Value = data.Results.(interface{})
				return true
			}
		}
		return false
	})
	if err != nil {
		rrp.Value = "system error because could not create waiter"
		return nil
	}
	l.Name = "GetSsoInfo HTTP Listener"
	defer l.Close()

	split := strings.Split(agg.ResourceURI, "IOMConfiguration")
	uri := split[0] + "Actions/GetSsoInfo"
	t.d.EventBus.PublishEvent(ctx, eh.NewEvent(ah.GenericActionEvent, &ah.GenericActionEventData{
		ID:          reqUUID,
		CmdID:       cmdID,
		ResourceURI: uri,
		ActionData:  map[string]interface{}{},
		Method:      "POST",
	}, time.Now()))

	_, werr := l.Wait(ctx)
	if werr != nil {
		rrp.Value = "system error because failed to get sso"
	}
	// Data has been copied in the listen call back so do nothing if it is good.

	return nil
}
