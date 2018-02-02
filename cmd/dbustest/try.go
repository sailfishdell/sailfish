package main

import (
    "fmt"
    "os"
    "log"

    "github.com/godbus/dbus"
)

var Object string = "xyz.openbmc_project.Software.Version"
var Interface string = "xyz.openbmc_project.Software.Version"
var Path dbus.ObjectPath = "/xyz/openbmc_project/software/14880bfa"
var Properties []string = []string{"Purpose", "Version"}

func main() {
	conn, err := dbus.SystemBus()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to connect to system bus:", err)
		os.Exit(1)
	}

	bo := conn.Object(Object, Path)

	variant, err := bo.GetProperty(Interface + "." + "Version")

	if err != nil {
		log.Fatalln("Error getting property:", err)
	}

	log.Println("Variant --->", variant.String())
}
