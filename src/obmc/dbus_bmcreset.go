// Build tags: only build this for the simulation build. Be sure to note the required blank line after.
// +build openbmc

package obmc

import (
	"context"
	"time"

	"github.com/godbus/dbus"
	eh "github.com/looplab/eventhorizon"
	ah "github.com/superchalupa/go-redfish/src/actionhandler"
	mydbus "github.com/superchalupa/go-redfish/src/dbus"
	"github.com/superchalupa/go-redfish/src/log"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

func BMCReset(ctx context.Context, event eh.Event, res *domain.HTTPCmdProcessedData, eb eh.EventBus) {
	logger := log.MustLogger("bmc_reset")

	bus := "org.openbmc.control.Bmc"
	path := "/org/openbmc/control/bmc0"
	intfc := "org.openbmc.control.Bmc"

	logger.Debug("resetType raw", "event", event.Data())
	ad := event.Data().(ah.GenericActionEventData)
	resetType, _ := ad.ActionData.(map[string]interface{})["ResetType"]
	call := "undefined"
	if resetType == "ForceRestart" {
		call = "coldReset"
	}
	if resetType == "GracefulRestart" {
		call = "warmReset"
	}
	logger.Info("Parsed reset type", "resetType", resetType)
	if call == "undefined" {
		return
	}

	statusCode := 200
	statusMessage := "OK"

	conn, err := dbus.SystemBus()
	if err != nil {
		logger.Error("Cannot connect to System Bus", "err", err)
		statusCode = 501
		statusMessage = "ERROR: Cannot attach to dbus system bus"
	}

	dh := mydbus.NewDbusHelper(conn, bus, path)
	timedctx, cancel := context.WithTimeout(ctx, time.Duration(DbusTimeout))
	defer cancel()
	_, err = dh.DbusCall(timedctx, 0, intfc+"."+call)
	if err != nil {
		statusCode = 501
		statusMessage = "Internal call failed"
	}

	eb.PublishEvent(ctx, eh.NewEvent(domain.HTTPCmdProcessed, domain.HTTPCmdProcessedData{
		CommandID:  event.Data().(ah.GenericActionEventData).CmdID,
		Results:    map[string]interface{}{"RESET": statusMessage},
		StatusCode: statusCode,
		Headers:    map[string]string{},
	}, time.Now()))
}
