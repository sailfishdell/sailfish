#!/bin/sh

set -e
set -x

scriptdir=$(cd $(dirname $0); pwd)
. ${scriptdir}/common-vars.sh

outname=${1:-event-list.json}

$CURLCMD --no-buffer -s $BASE/events | perl -n -e 'print if /^data:/' | cut -d: -f2- | perl -p -i -e 's/^\s*//;g' | grep -v RedfishEvent | grep -v "RedfishResource:created" | grep -v "RedfishResource:removed" | tee -a $outname
