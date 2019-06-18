package stdmeta

import (
	"context"
	"strings"
	"time"

	eh "github.com/looplab/eventhorizon"
	ah "github.com/superchalupa/sailfish/src/actionhandler"
	"github.com/superchalupa/sailfish/src/dell-resources/dm_event"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

var (
	certInfoPlugin = domain.PluginType("certinfo")
)

const (
	RequestGetCertInfo = eh.EventType("Request:GetCertInfo")
)

func InitializeCertInfo(d *domain.DomainObjects) {
	domain.RegisterPlugin(func() domain.Plugin { return &certInfo{d: d} })
}

type certInfo struct {
	d *domain.DomainObjects
}

func (t *certInfo) PluginType() domain.PluginType { return certInfoPlugin }

/*
   Oh the tangled web we must spin....

   Setting the wayback machine to EC 1.0, lets describe the workflow for installing an identy cert.
   1st - Request a CSR from the EC, note the EC's location and use appropriate CMC.Integrated.x address depending on
       installed slot (will use slot 2 ec as example) but in reality they both do the same thing since this only works
       on the active EC

       POST /redfish/v1/Managers/CMC.Integrated.2/CertificateService/Actions/DellCertificateService.GenerateCSR '{"Type" : "FactoryIdentity"}'

   2nd - Since the CSR is not returned in the response (instead get a 204), read it directly

       GET https://localhost/redfish/v1/Managers/CMC.Integrated.2/CertificateService

   3rd - the caller must extract the CSR from the HTTP response of the GET, and then send it off to get it signed.

   4th - Upload the signed cert

       POST https://localhost/redfish/v1/Managers/CMC.Integrated.2/CertificateService/CertificateInventory/FactoryIdentity.1 '{ "Certificate" : "-----BEGIN CERTIFICATE-----\n ... \n-----END CERTIFICATE-----" }'

   5th - Now to Verify the cert is uploaded do a GET on the same URL as the upload

       GET https://localhost/redfish/v1/Managers/CMC.Integrated.2/CertificateService/CertificateInventory/FactoryIdentity.1

   6th - since there is no action to actually verify the cert is valid, just keep calling the same URL and GET the cert, if it comes back null it isn't valid

   ... so now the stage is set

   The block of code below is for displaying the cert and impacts #5 and #6, because the cert is not stored on the file system a SMIL call triggers the backend
   to read of the cert and (re)write a specific file *tada*.  This is done each time the SMIL call is made.  So on every GET request Sailfish must send a request
   to the pump which calls the SMIL call again.  This triggers a (re)write of the file and since the pump is using inotify on the that file, will result in a
   HTTPCmdProcessed (HTTP 204) + a FileReadEvent.  BUT there is the possiblity that the SMIL call will fail, if that occurs the pump will return with ONLY
   the HTTPCmdProcessed (HTTP 500).

   So the event waiter must listen for both types of events, and set/clear the certificate depending on the response.

*/
func (t *certInfo) PropertyGet(
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
		if event.EventType() != dm_event.FileReadEvent && event.EventType() != domain.HTTPCmdProcessed {
			return false
		}
		if data, ok := event.Data().(*dm_event.FileReadEventData); ok {
			// make sure this file was the factory cert on this URI
			split := strings.Split(agg.ResourceURI, "/")
			if len(split) >= 7 && split[4] == data.FQDD && split[7] == data.URI {
				rrp.Value = data.Content
				return true
			}
		}
		if data, ok := event.Data().(*domain.HTTPCmdProcessedData); ok {
			if data.CommandID == cmdID && data.StatusCode != 204 {
				rrp.Value = nil
				// TODO: May need to return an error to the original response??
				// for now just Null out the value and continue
				return true
			}
		}
		return false
	})
	if err != nil {
		rrp.Value = "system error because could not create waiter"
		return nil
	}
	l.Name = "GetCertInfo HTTP Listener"
	defer l.Close()

	uri := agg.ResourceURI + "/Actions/GetCertInfo"
	t.d.EventBus.PublishEvent(ctx, eh.NewEvent(ah.GenericActionEvent, &ah.GenericActionEventData{
		ID:          reqUUID,
		CmdID:       cmdID,
		ResourceURI: uri,
		ActionData:  map[string]interface{}{},
		Method:      "POST",
	}, time.Now()))

	_, werr := l.Wait(ctx)
	if werr != nil {
		rrp.Value = nil
	}
	// Data has been copied in the listen call back so do nothing if it is good.

	return nil
}
