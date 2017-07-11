#!/bin/sh

set -e
set -x

unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy

scriptdir=$(cd $(dirname $0); pwd)
outputdir=${1:-out/}
skiplist=${2:-}

HOST=${HOST:-localhost}
PORT=${PORT:-8080}
if [ "${PORT}" = "443" -o "${PORT}" = "8443" ]; then
    PROTO=https
else
    PROTO=http
fi
BASE=${PROTO}://${HOST}:${PORT}
START_URL=${START_URL:-"/redfish/v1/"}

AUTH_HEADER=${AUTH_HEADER:-foo: bar}
if [ -n "$TOKEN" ]; then
    AUTH_HEADER="Authorization: Bearer $TOKEN"
fi

rm -rf ${outputdir}/ && mkdir ${outputdir}
echo $START_URL | sort |uniq > ${outputdir}/to-visit.txt

# for information only
echo ${BASE}${url} > ${outputdir}/url.txt

LOOPS=0
POTENTIALLY_GOT_MORE=1
while [ $POTENTIALLY_GOT_MORE -eq 1 ]
do
    POTENTIALLY_GOT_MORE=0
    for url in $(cat ${outputdir}/to-visit.txt)
    do
        if grep -q "^${url}\$" ${outputdir}/visited.txt ${outputdir}/errors.txt $skiplist; then
            continue
        fi
        OUTFILE=${outputdir}/$( echo -n ${url} | perl -p -e 's/[^a-zA-Z0-9]/_/g;' ).json
        if ! curl -f -H "$AUTH_HEADER" -s ${CURL_OPTS} -L ${BASE}${url}  -o $OUTFILE ; then
            echo $url >> ${outputdir}/errors.txt
            continue
        fi
        cat $OUTFILE | jq -r 'recurse (.[]?) | objects | select(has("@odata.id")) | .["@odata.id"]' >> ${outputdir}/to-visit.txt ||:
        POTENTIALLY_GOT_MORE=1
        echo $url >> ${outputdir}/visited.txt
    done
    cat ${outputdir}/to-visit.txt | sort | uniq > ${outputdir}/to-visit-new.txt
    mv ${outputdir}/to-visit-new.txt ${outputdir}/to-visit.txt
    cat ${outputdir}/visited.txt | sort | uniq > ${outputdir}/visited-new.txt
    mv ${outputdir}/visited-new.txt  ${outputdir}/visited.txt
    LOOPS=$(( LOOPS + 1 ))
done

time curl ${CURL_OPTS} -i -H "$AUTH_HEADER" -s -L -w"\nTotal request time: %{time_total} seconds for url: %{url_effective}\n" $(cat ${outputdir}/visited.txt | perl -n -e "print '${BASE}' . \$_" ) | tee ${outputdir}/entire-tree.txt

echo "Took $LOOPS loops to collect the URL list"
