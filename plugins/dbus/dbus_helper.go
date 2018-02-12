package dbus

import (
	"context"
	"fmt"

	"github.com/godbus/dbus"
)

type dbus_helper struct {
	object dbus.BusObject
}

func NewDbusHelper(conn *dbus.Conn, bus, path string) dbus_helper {
	return dbus_helper{
		object: conn.Object(bus, dbus.ObjectPath(path)),
	}
}

func (m *dbus_helper) DbusCall(ctx context.Context, flags dbus.Flags, fn string, args ...interface{}) (*dbus.Call, error) {
	callObj := m.object.Call(fn, flags, args...)
	if callObj.Err != nil {
		fmt.Printf("Pre-error: %s\n", callObj.Err.Error())
		return callObj, callObj.Err
	}

	select {
	case <-callObj.Done:
		fmt.Printf("donecall. err: %s return: %s\n", callObj.Err.Error(), callObj.Body)
	case <-ctx.Done():
		// give up (use with context.WithTimeout() or context.WithDeadline() to implement a timeout)
	}

	return callObj, callObj.Err
}
