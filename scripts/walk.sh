#!/bin/bash

set -e
set -x

scriptdir=$(cd $(dirname $0); pwd)
. ${scriptdir}/common-vars.sh

set_auth_header

outputdir=${1:-out/}
skiplist=${2:-}

timingarg="\nTotal request time: %{time_total} seconds for url: %{url_effective}\n"

rm -rf ${outputdir}/ && mkdir ${outputdir}
LOGFILE=$outputdir/script-output.txt
exec 1> >(exec -a 'LOGGING TEE' tee $LOGFILE) 2>&1
TEEPID=$!

cleanup() {
    # close FDs to ensure tee finishes
    exec 1>&0 2>&1
    if [ -n "$TEEPID" ];then
        while ps --pid $TEEPID > /dev/null 2>&1
        do
            sleep 1
        done
    fi
}
trap 'cleanup' EXIT

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
        if ! $CURLCMD --fail -H "$AUTH_HEADER" --silent -L -w"$timingarg" ${BASE}${url} -o ${OUTFILE}-RAW ; then
            echo $url >> ${outputdir}/errors.txt
            continue
        fi
        cat ${OUTFILE}-RAW | jq . > ${OUTFILE}
        rm ${OUTFILE}-RAW
        cat $OUTFILE | jq -r 'recurse (.[]?) | objects | select(has("@odata.id")) | .["@odata.id"]' | perl -p -i -e 's/(\/#.*)//' | perl -p -i -e 's/(#.*)//' | grep -v JSONSchema >> ${outputdir}/to-visit.txt ||:
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
time $CURLCMD --fail -i -H "$AUTH_HEADER" -w"$timingarg" -s $(cat ${outputdir}/visited.txt | perl -n -e "print '${BASE}' . \$_" ) | tee ${outputdir}/entire-tree.txt

echo "Took $LOOPS loops to collect the URL list"

