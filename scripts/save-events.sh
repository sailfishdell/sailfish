#!/bin/sh

set -e
set -x

scriptdir=$(cd $(dirname $0); pwd)
. ${scriptdir}/common-vars.sh

outname=${1:-event-list.json}

$CURLCMD -s $BASE/events | perl -n -e 'print unless /^\s*$/' | cut -d: -f2- | perl -p -i -e 's/^\s*//;g' tee -a $outname
