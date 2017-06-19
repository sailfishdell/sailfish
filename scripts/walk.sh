#!/bin/sh

set -e
set -x

scriptdir=$(cd $(dirname $0); pwd)

HOST=${HOST:-localhost}
PORT=${PORT:-8080}
if [ "${PORT}" = "443" -o "${PORT}" = "8443" ]; then
    PROTO=https
else
    PROTO=http
fi
BASE=${PROTO}://${HOST}:${PORT}
START_URL="/redfish/v1/"

if [ -n "$TOKEN" ]; then
    AUTH_HEADER="-H 'Authentication: bearer $TOKEN'"
fi

echo $START_URL | sort |uniq > URLS-to-visit.txt
rm -f URLS-visited.txt

LOOPS=0
while ! diff -u URLS-to-visit.txt URLS-visited.txt
do
    for url in $(cat URLS-to-visit.txt)
    do
        if grep -q "^${url}\$" URLS-visited.txt; then
            continue
        fi
        curl $AUTH_HEADER -s -L ${BASE}${url} | jq -r 'recurse (.[]?) | objects | select(has("@odata.id")) | .["@odata.id"]' >> URLS-to-visit.txt
        echo $url >> URLS-visited.txt
    done
    cat URLS-to-visit.txt | sort | uniq > URLS-to-visit-new.txt
    mv URLS-to-visit-new.txt URLS-to-visit.txt
    cat URLS-visited.txt | sort | uniq > URLS-visited-new.txt
    mv URLS-visited-new.txt  URLS-visited.txt
    LOOPS=$(( LOOPS + 1 ))
done

time curl $AUTH_HEADER -s -L -w"\nTotal request time: %{time_total} seconds\n" $(cat URLS-visited.txt | perl -n -e "print '${BASE}' . \$_" )

echo "Took $LOOPS loops to collect the URL list"
