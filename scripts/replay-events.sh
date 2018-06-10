#!/bin/sh

set -e
set -x

scriptdir=$(cd $(dirname $0); pwd)
. ${scriptdir}/common-vars.sh

if [ -z "$1" ]; then
    echo "need to specify event file"
    exit 1
fi

cat $1 | while read line ; do $CURLCMD -f $BASE/api/Event%3AInject -d  "$line"; done
