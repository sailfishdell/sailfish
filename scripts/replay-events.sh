#!/bin/sh

set -e
set -x

# new default 8080 port for this for speed
port=${port:-8080}

scriptdir=$(cd $(dirname $0); pwd)
. ${scriptdir}/common-vars.sh

if [ -z "$1" ]; then
    echo "need to specify event file"
    exit 1
fi

cat $1 | while read line ; do $CURLCMD --fail -f $BASE/api/Event%3AInject -d  "$line"; done
