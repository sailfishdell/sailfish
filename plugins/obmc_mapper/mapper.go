package obmc_mapper

import (
	"context"
	"fmt"
	"time"

	"github.com/godbus/dbus"
)

type mapper struct {
	conn         *dbus.Conn
	mapperObject dbus.BusObject
}

const (
	MapperBusName    = "xyz.openbmc_project.ObjectMapper"
	MapperObjectPath = "/xyz/openbmc_project/object_mapper"
	MapperInterface  = "xyz.openbmc_project.ObjectMapper"

	getSubTreePaths string = MapperInterface + ".GetSubTreePaths"
	getSubTree      string = MapperInterface + ".GetSubTree"
	getObject       string = MapperInterface + ".GetObject"
	getAncestors    string = MapperInterface + ".GetAncestors"

	DbusTimeout = 10
)

func New(conn *dbus.Conn) *mapper {
	mo := conn.Object(MapperBusName, MapperObjectPath)

	return &mapper{
		conn:         conn,
		mapperObject: mo,
	}
}

func (m *mapper) GetAncestors(ctx context.Context, path string, intfcs ...string) ([]interface{}, error) {
	// sas : path, array of interfaces
	timedctx, cancel := context.WithTimeout(ctx, time.Duration(DbusTimeout))
	defer cancel()
	call, err := m.DbusCall(timedctx, dbus.Flags(0), getAncestors, path, intfcs)
	return call.Body, err
}
func (m *mapper) GetObject(ctx context.Context, path string, intfcs ...string) ([]interface{}, error) {
	// sas : path, array of interfaces
	timedctx, cancel := context.WithTimeout(ctx, time.Duration(DbusTimeout))
	defer cancel()
	call, err := m.DbusCall(timedctx, dbus.Flags(0), getObject, path, intfcs)
	return call.Body, err
}

func (m *mapper) GetSubTreePaths(ctx context.Context, path string, depth int, interfaces ...string) ([]interface{}, error) {
	// sias : path, depth (0-unlimited), array of interfaces
	timedctx, cancel := context.WithTimeout(ctx, time.Duration(DbusTimeout))
	defer cancel()
	call, err := m.DbusCall(timedctx, dbus.Flags(0), getSubTreePaths, path, depth, interfaces)
	return call.Body, err
}

func (m *mapper) GetSubTree(ctx context.Context, path string, depth int, interfaces ...string) ([]interface{}, error) {
	// sias : path, depth (0-unlimited), array of interfaces
	timedctx, cancel := context.WithTimeout(ctx, time.Duration(DbusTimeout))
	defer cancel()
	call, err := m.DbusCall(timedctx, dbus.Flags(0), getSubTree, path, depth, interfaces)
	return call.Body, err
}

func (m *mapper) DbusCall(ctx context.Context, flags dbus.Flags, fn string, args ...interface{}) (*dbus.Call, error) {

	callObj := m.mapperObject.Call(fn, flags, args...)
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
