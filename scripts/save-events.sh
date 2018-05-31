#!/bin/sh

set -e
set -x

scriptdir=$(cd $(dirname $0); pwd)
. ${scriptdir}/common-vars.sh

$CURLCMD -s $BASE/events | perl -n -e 'print unless /^\s*$/' | cut -d: -f2- | tee -a event-list.json
