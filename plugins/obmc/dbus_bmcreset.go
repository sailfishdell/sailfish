// Build tags: only build this for the simulation build. Be sure to note the required blank line after.
// +build openbmc

package obmc

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/godbus/dbus"
	eh "github.com/looplab/eventhorizon"
	ah "github.com/superchalupa/go-redfish/plugins/actionhandler"
	mydbus "github.com/superchalupa/go-redfish/plugins/dbus"
	domain "github.com/superchalupa/go-redfish/redfishresource"
)

func BMCReset(ctx context.Context, event eh.Event, res *domain.HTTPCmdProcessedData, eb eh.EventBus) {
	bus := "org.openbmc.control.Bmc"
	path := "/org/openbmc/control/bmc0"
	intfc := "org.openbmc.control.Bmc"

	fmt.Printf("parse resetType: %s\n", event.Data())
	ad := event.Data().(ah.GenericActionEventData)
	resetType, _ := ad.ActionData.(map[string]interface{})["ResetType"]
	call := "undefined"
	if resetType == "ForceRestart" {
		call = "coldReset"
	}
	if resetType == "GracefulRestart" {
		call = "warmReset"
	}
	fmt.Printf("\tgot: %s\n", resetType)
	if call == "undefined" {
		return
	}

	fmt.Printf("connect to system bus\n")
	statusCode := 200
	statusMessage := "OK"

	conn, err := dbus.SystemBus()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot connect to System Bus: %s\n", err.Error())
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

	eb.HandleEvent(ctx, eh.NewEvent(domain.HTTPCmdProcessed, domain.HTTPCmdProcessedData{
		CommandID:  event.Data().(ah.GenericActionEventData).CmdID,
		Results:    map[string]interface{}{"RESET": statusMessage},
		StatusCode: statusCode,
		Headers:    map[string]string{},
	}, time.Now()))
}
