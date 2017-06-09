#!/bin/sh

set -e
#set -x

scriptdir=$(cd $(dirname $0); pwd)

BASE=http://localhost:8080
URLS_NEW="$BASE/redfish/v1/"
URLS=

while [ "$URLS_NEW" != "$URLS" ]
do
    URLS="$URLS_NEW"
    URLS_NEW=$(curl -s -L $URLS | jq -r 'recurse (.[]?) | objects | select(has("@odata.id")) | .["@odata.id"]' | sort | uniq | perl -n -e "print  \"$BASE\" . \$_" )
done

#curl -s -L $URLS

time curl -s -L -w"\nTotal request time: %{time_total} seconds\n" $URLS
