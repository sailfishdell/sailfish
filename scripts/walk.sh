#!/bin/sh

set -e
set -x

unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy

scriptdir=$(cd $(dirname $0); pwd)

HOST=${HOST:-localhost}
PORT=${PORT:-8080}
if [ "${PORT}" = "443" -o "${PORT}" = "8443" ]; then
    PROTO=https
else
    PROTO=http
fi
BASE=${PROTO}://${HOST}:${PORT}
START_URL=${START_URL:-"/redfish/v1/"}

AUTH_HEADER="foo: bar"
if [ -n "$TOKEN" ]; then
    AUTH_HEADER="Authorization: Bearer $TOKEN"
fi

echo $START_URL | sort |uniq > URLS-to-visit.txt
rm -f URLS-visited.txt
rm -rf out/
mkdir out

LOOPS=0
TRIEDONE=1
while [ $TRIEDONE -eq 1 ]
do
    TRIEDONE=0
    for url in $(cat URLS-to-visit.txt)
    do
        if grep -q "^${url}\$" URLS-visited.txt; then
            continue
        fi
        TRIEDONE=1
        OUTFILE=out/$( echo -n ${url} | perl -p -e 's/[^a-zA-Z0-9]/_/g;' ).json
        curl -H "$AUTH_HEADER" -s -L ${BASE}${url}  > $OUTFILE || continue
        cat $OUTFILE | jq -r 'recurse (.[]?) | objects | select(has("@odata.id")) | .["@odata.id"]' >> URLS-to-visit.txt ||:
        echo $url >> URLS-visited.txt
    done
    cat URLS-to-visit.txt | sort | uniq > URLS-to-visit-new.txt
    mv URLS-to-visit-new.txt URLS-to-visit.txt
    cat URLS-visited.txt | sort | uniq > URLS-visited-new.txt
    mv URLS-visited-new.txt  URLS-visited.txt
    LOOPS=$(( LOOPS + 1 ))
done

time curl -H "$AUTH_HEADER" -s -L -w"\nTotal request time: %{time_total} seconds for url: %{url_effective}\n" $(cat URLS-visited.txt | perl -n -e "print '${BASE}' . \$_" ) | tee out/entire-tree.txt

echo "Took $LOOPS loops to collect the URL list"
