package dbus

import (
	"context"
	"time"

	"github.com/godbus/dbus"
)

type mapper struct {
	dh dbus_helper
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
	return &mapper{dh: NewDbusHelper(conn, MapperBusName, MapperObjectPath)}
}

func (m *mapper) GetAncestors(ctx context.Context, path string, intfcs ...string) ([]interface{}, error) {
	// sas : path, array of interfaces
	timedctx, cancel := context.WithTimeout(ctx, time.Duration(DbusTimeout))
	defer cancel()
	call, err := m.dh.DbusCall(timedctx, dbus.Flags(0), getAncestors, path, intfcs)
	return call.Body, err
}
func (m *mapper) GetObject(ctx context.Context, path string, intfcs ...string) ([]interface{}, error) {
	// sas : path, array of interfaces
	timedctx, cancel := context.WithTimeout(ctx, time.Duration(DbusTimeout))
	defer cancel()
	call, err := m.dh.DbusCall(timedctx, dbus.Flags(0), getObject, path, intfcs)
	return call.Body, err
}

func (m *mapper) GetSubTreePaths(ctx context.Context, path string, depth int, interfaces ...string) ([]interface{}, error) {
	// sias : path, depth (0-unlimited), array of interfaces
	timedctx, cancel := context.WithTimeout(ctx, time.Duration(DbusTimeout))
	defer cancel()
	call, err := m.dh.DbusCall(timedctx, dbus.Flags(0), getSubTreePaths, path, depth, interfaces)
	return call.Body, err
}

func (m *mapper) GetSubTree(ctx context.Context, path string, depth int, interfaces ...string) ([]interface{}, error) {
	// sias : path, depth (0-unlimited), array of interfaces
	timedctx, cancel := context.WithTimeout(ctx, time.Duration(DbusTimeout))
	defer cancel()
	call, err := m.dh.DbusCall(timedctx, dbus.Flags(0), getSubTree, path, depth, interfaces)
	return call.Body, err
}
