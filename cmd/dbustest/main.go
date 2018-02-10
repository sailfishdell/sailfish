package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/godbus/dbus"
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
	ctx := context.Background()
	conn, err := dbus.SystemBus()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to connect to system bus:", err)
		os.Exit(1)
	}

	m := New(conn)
	ret, err := m.GetSubTreePaths(ctx, "/xyz/openbmc_project", 0, "xyz.openbmc_project.Sensor.Value")
	if err == nil {
		b, _ := json.MarshalIndent(ret, "", "  ")
		fmt.Printf("DBUS CALL RETURN: %s\n", b)
	} else {
		fmt.Printf("Error from dbus call: %s\n", err.Error())
	}

	//m.GetSubTree(ctx,  "/xyz/openbmc_project", 0)

	/*
		busObject := conn.Object(BusName, Path)
		for _, p := range Properties {
			variant, err := busObject.GetProperty(Interface + "." + p)
			if err != nil {
				log.Fatalln("Error getting property:", err)
				continue
			}
			log.Println("Variant --->", variant.String())
		}
	*/
}
