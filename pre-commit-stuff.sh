#!/bin/sh

golint commands/mockserver/  src/mockbackend/  src/redfishserver/

go fmt  \
    github.com/superchalupa/go-redfish/commands/mockserver  \
    github.com/superchalupa/go-redfish/src/redfishserver/   \
    github.com/superchalupa/go-redfish/src/mockbackend/
