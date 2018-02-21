#!/bin/sh

set -e
set -x

export user=${user:-Administrator}
export pass=${pass:-password}
export host=${host:-localhost}
export port=${port:-8443}
export prot=${prot:-https}
BASE=${prot}://${host}:${port}
test_time=${test_time:-20s}

# idrac
export CURL_OPTS=-k

if [ "${host}" = "localhost" ]; then
    cacert=${cacert:-./ca.crt}
else
    cacert=${cacert:-./${host}-ca.crt}
fi

eval $(scripts/login.sh $user $pass)

topout=out-${host}:${port}
rm -rf ${topout}
mkdir ${topout}
scripts/walk.sh ${topout} > ${topout}/walk.txt 2>&1

for url in $(cat ${topout}/visited.txt)
do
    outdir=${topout}/$(echo $url | perl -p -i -e 's/[^a-z0-9A-Z.]/_/g;')
    mkdir $outdir
    for concurrent in $(seq 1 20)
    do
        FN_concurrent=$(printf "%03d" $concurrent)

        hey -c $concurrent -z ${test_time} -a ${user}:${pass} ${BASE}$url > $outdir/bench-basicauth-c${FN_concurrent}-z${test_time}.txt 2>&1
        if [ -e STOP ]; then
            rm STOP
            exit 0
        fi

        hey -c $concurrent -z ${test_time} -H "X-Auth-Token: ${X_AUTH_TOKEN}" ${BASE}/$url > $outdir/bench-tokenauth-c${FN_concurrent}-z${test_time}.txt 2>&1

        if [ -e STOP ]; then
            rm STOP
            exit 0
        fi
        sleep $test_time
        if [ -e STOP ]; then
            rm STOP
            exit 0
        fi
    done
done
