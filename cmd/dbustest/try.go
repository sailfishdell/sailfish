package main

import (
	"fmt"
	"log"
	"os"

	"github.com/godbus/dbus"
)

var BusName string = "xyz.openbmc_project.Software.Version"
var Interface string = "xyz.openbmc_project.Software.Version"
var Path dbus.ObjectPath = "/xyz/openbmc_project/software/14880bfa"
var Properties []string = []string{"Purpose", "Version"}

func main() {
	conn, err := dbus.SystemBus()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to connect to system bus:", err)
		os.Exit(1)
	}

	busObject := conn.Object(BusName, Path)

	for _, p := range Properties {
		variant, err := busObject.GetProperty(Interface + "." + p)
		if err != nil {
			log.Fatalln("Error getting property:", err)
			continue
		}
		log.Println("Variant --->", variant.String())
	}
}
