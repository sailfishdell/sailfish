#!/bin/sh
set -e
set -x

go build "$@" github.com/superchalupa/go-redfish/cmd/ocp-server

set +x
echo -e "\nBUILD SUCCES: binary ready: ./ocp-server"
