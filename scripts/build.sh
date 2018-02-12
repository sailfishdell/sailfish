#!/bin/sh
scriptdir=$(cd $(dirname $0); pwd)
cd $scriptdir/..

set -e
set -x

binaries=${binaries:-$(find cmd/* -type d)}

for cmd in ${binaries}
do
    go build "$@" github.com/superchalupa/go-redfish/$cmd
done

set +x
echo -e "\nBUILD SUCCES: binary ready: ./ocp-server"
