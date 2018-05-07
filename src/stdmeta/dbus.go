package stdmeta

import (
	"context"
	"fmt"
	"os"

	"github.com/godbus/dbus"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

var (
	dbusPropertyPlugin = domain.PluginType("dbus_property")
)

func init() {
	conn, err := dbus.SystemBus()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot register dbus_property plugin, could not connect to System Bus: %s\n", err.Error())
		return
	}
	domain.RegisterPlugin(func() domain.Plugin { return &dbusProperty{conn: conn} })
}

type dbusProperty struct {
	conn *dbus.Conn
}

func (t *dbusProperty) PluginType() domain.PluginType { return dbusPropertyPlugin }

func (t *dbusProperty) PropertyGet(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) {

	// morally equivalent to do{}while(0)
	var ok bool
	var bus, intfc, path, prop string
	for once := true; once; once = false {
		bus, ok = meta["bus_name"].(string)
		if !ok {
			break
		}

		intfc, ok = meta["interface_name"].(string)
		if !ok {
			break
		}

		path, ok = meta["path"].(string)
		if !ok {
			break
		}

		prop, ok = meta["property"].(string)
		if !ok {
			break
		}
	}

	if !ok {
		fmt.Printf("Misconfigured runcmd plugin, required value not set\n")
		return
	}

	busObject := t.conn.Object(bus, dbus.ObjectPath(path))
	variant, err := busObject.GetProperty(intfc + "." + prop)
	if err != nil {
		fmt.Printf("Error getting property: %s\n", err.Error())
		return
	}

	rrp.Value = variant.Value()
}
