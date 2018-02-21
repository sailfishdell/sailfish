#!/bin/sh

set -e
set -x

unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy

user=${user:-Administrator}
pass=${pass:-password}
host=${host:-localhost}
port=${port:-8443}

if [ "${host}" = "localhost" ]; then
    cacert=${cacert:-./ca.crt}
else
    cacert=${cacert:-./${host}-ca.crt}
fi

scriptdir=$(cd $(dirname $0); pwd)
outputdir=${1:-out/}
skiplist=${2:-}

eval $(scripts/login.sh $user $pass)

host=${host:-localhost}
if [ "${port}" = "443" -o "${port}" = "8443" ]; then
    prot=${prot:-https}
else
    prot=${prot:-http}
fi
BASE=${prot}://${host}:${port}
START_URL=${START_URL:-"/redfish/v1"}

AUTH_HEADER=${AUTH_HEADER:-}
if [ -n "$TOKEN" ]; then
    AUTH_HEADER="Authorization: Bearer $TOKEN"
fi

timingarg="\nTotal request time: %{time_total} seconds for url: %{url_effective}\n"
CURLCMD="curl --cacert ${cacert} ${CURL_OPTS} -L "

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
        OUTFILE=${outputdir}/$( echo -n ${url} | perl -p -e 's/[^a-zA-Z0-9.]/_/g;' ).json
        if ! $CURLCMD --fail -H "$AUTH_HEADER" --silent -L -w"$timingarg" ${BASE}${url}  -o $OUTFILE ; then
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

timingarg="\nTotal PIPELINED request time (for this subrequest): %{time_total} seconds for url: %{url_effective}\n"
time $CURLCMD -i -H "$AUTH_HEADER" -w"$timingarg" -s $(cat ${outputdir}/visited.txt | perl -n -e "print '${BASE}' . \$_" ) | tee ${outputdir}/entire-tree.txt

echo "Took $LOOPS loops to collect the URL list"
