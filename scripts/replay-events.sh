#!/bin/sh

set -e
set -x

scriptdir=$(cd $(dirname $0); pwd)
. ${scriptdir}/common-vars.sh

cat event-list.json | while read line ; do $CURLCMD $BASE/api/Event%3AInject -d  "$line"; done
