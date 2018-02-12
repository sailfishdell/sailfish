package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/godbus/dbus"
	mapper "github.com/superchalupa/go-redfish/plugins/dbus"
)

// Get BMC UUID:
//     resp = mapper.get_subtree( path=INVENTORY_ROOT, interfaces=[CHS_INTF_NAME])
//          INVENTORY_ROOT = '/xyz/openbmc_project/inventory'
//          CHS_INTF_NAME = 'xyz.openbmc_project.Common.UUID'
// gets conn, path
// use that

var BusName string = "xyz.openbmc_project.Software.Version"
var Interface string = "xyz.openbmc_project.Software.Version"
var Path dbus.ObjectPath = "/xyz/openbmc_project/software/14880bfa"
var Properties []string = []string{"Purpose", "Version"}

func main() {
	var interfaces strslice

	GetSubTreeCMD := flag.NewFlagSet("GetSubTree", flag.ExitOnError)
	GSTpathPtr := GetSubTreeCMD.String("p", "/xyz/openbmc_project", "which path to query")
	GSTdepthPtr := GetSubTreeCMD.Int("d", 0, "Search depth for subtree queries")
	GetSubTreeCMD.Var(&interfaces, "i", "Interface list: ex. xyz.openbmc_project.Sensor.Value")

	GetSubTreePathsCMD := flag.NewFlagSet("GetSubTreePaths", flag.ExitOnError)
	GSTPpathPtr := GetSubTreePathsCMD.String("p", "/xyz/openbmc_project", "which path to query")
	GSTPdepthPtr := GetSubTreePathsCMD.Int("d", 0, "Search depth for subtree queries")
	GetSubTreePathsCMD.Var(&interfaces, "i", "Interface list: ex. xyz.openbmc_project.Sensor.Value")

	GetObjectCMD := flag.NewFlagSet("GetObject", flag.ExitOnError)
	GOpathPtr := GetObjectCMD.String("p", "/xyz/openbmc_project", "which path to query")
	GetObjectCMD.Var(&interfaces, "i", "Interface list: ex. xyz.openbmc_project.Sensor.Value")

	GetAncestorsCMD := flag.NewFlagSet("GetAncestors", flag.ExitOnError)
	GApathPtr := GetAncestorsCMD.String("p", "/xyz/openbmc_project", "which path to query")
	GetAncestorsCMD.Var(&interfaces, "i", "Interface list: ex. xyz.openbmc_project.Sensor.Value")

	ctx := context.Background()
	conn, err := dbus.SystemBus()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to connect to system bus:", err)
		os.Exit(1)
	}

	m := mapper.New(conn)

	var ret []interface{}

	help := func() {
		fmt.Printf("usage: " + os.Args[0] + " <command> [args]...\n")
		os.Exit(1)
	}
	if len(os.Args) < 2 {
		help()
	}

	switch os.Args[1] {
	case "GetSubTree":
		GetSubTreeCMD.Parse(os.Args[2:])
		ret, err = m.GetSubTree(ctx, *GSTpathPtr, *GSTdepthPtr, interfaces...)

	case "GetSubTreePaths":
		GetSubTreePathsCMD.Parse(os.Args[2:])
		ret, err = m.GetSubTreePaths(ctx, *GSTPpathPtr, *GSTPdepthPtr, interfaces...)

	case "GetObject":
		GetObjectCMD.Parse(os.Args[2:])
		ret, err = m.GetObject(ctx, *GOpathPtr, interfaces...)

	case "GetAncestors":
		GetAncestorsCMD.Parse(os.Args[2:])
		ret, err = m.GetAncestors(ctx, *GApathPtr, interfaces...)

	default:
		help()
	}

	if err == nil {
		b, _ := json.MarshalIndent(ret, "", "  ")
		fmt.Printf("DBUS CALL RETURN: %s\n", b)
	} else {
		fmt.Printf("Error from dbus call: %s\n", err.Error())
	}

}

// Define a type named "strslice" as a slice of strings
type strslice []string

// Now, for our new type, implement the two methods of
// the flag.Value interface...
// The first method is String() string
func (i *strslice) String() string {
	return fmt.Sprintf("%v", *i)
}

// The second method is Set(value string) error
func (i *strslice) Set(value string) error {
	*i = append(*i, value)
	return nil
}
