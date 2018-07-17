#!/bin/sh

set -x
set -e

unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy

scriptdir=$(cd $(dirname $0); pwd)
. ${scriptdir}/common-vars.sh

outputdir=${1:-out/}
skiplist=${2:-}
runtoken=${runtoken:-1}
runbasic=${runbasic:-0}

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

if [ ! -e $outputdir/to-visit.txt ]; then
    tempdir=$(mktemp -d ./output-XXXXXX)
    trap 'cleanup; [ -n "$tepdir" ] && rm -rf $tempdir' EXIT

    . $scriptdir/walk.sh $tempdir
    outputdir=${1:-out/}

    mkdir -p $outputdir/
    cat $tempdir/errors.txt $tempdir/to-visit.txt | sort | uniq -u | grep -v /redfish/v1/SessionService/Sessions/  > $outputdir/to-visit.txt  ||:

    rm -rf $tempdir
fi

rm -rf $outputdir/{token,basic}
mkdir -p $outputdir $outputdir/token $outputdir/basic
LOGFILE=$outputdir/script-output.txt
exec 1> >(exec -a 'LOGGING TEE' tee $LOGFILE) 2>&1
TEEPID=$!

cat $outputdir/to-visit.txt | perl -p -i -e "s|^|GET ${BASE}|" > $outputdir/vegeta-targets.txt
cat $outputdir/to-visit.txt | perl -p -i -e "s|^|GET ${prot}://${user}:${pass}\@${host}:${port}|" > $outputdir/basic/vegeta-targets.txt

set_auth_header

if [ -n "${cacert_file}" ] ;then
    cert_opt="-root-certs $cacert_file"
else
    cert_opt="-insecure"
fi

echo "Running vegeta"

time=10s

savetop() {
    echo "Starting TOP for vegeta run. RATE: $index" > $1
    ssh root@${host} 'top -b -d1 -o %MEM' >> $1 &
    SSHPID=$!
}

for i in $(seq 30 ) $(seq 40 10 100) ; do
    index=$(printf "%03d" $i)

    if [ ${runtoken} == 1 ]; then
        [ "${profile}" == 1 ] && savetop $outputdir/token/results-r${index}-CPU.txt
        vegeta attack -targets $outputdir/vegeta-targets.txt -output $outputdir/token/results-rate-${index}.bin -header "$AUTH_HEADER" -duration=${time} $cert_opt -rate $i
        [ -n "$SSHPID" ] && kill $SSHPID ||:

        cat $outputdir/token/results-rate-${index}.bin | vegeta report -reporter text > $outputdir/token/report-r${index}-text.txt
        cat $outputdir/token/results-rate-${index}.bin | vegeta report -reporter plot > $outputdir/token/report-r${index}-plot.html
        cat $outputdir/token/results-rate-${index}.bin | vegeta report -reporter='hist[0,2ms,4ms,6ms,8ms,10ms,20ms,30ms,40ms,60ms,80ms,100ms,200ms,400ms,800ms,1600ms,3200ms,6400ms]' > $outputdir/token/report-r${index}-hist.txt
    fi

    if [ ${runbasic} == 1 ]; then
        [ "${profile}" == 1 ] && savetop $outputdir/basic/results-r${index}-CPU.txt
        vegeta attack -targets $outputdir/basic/vegeta-targets.txt -output $outputdir/basic/results-rate-${index}.bin -duration=${time} $cert_opt -rate $i
        [ -n "$SSHPID" ] && kill $SSHPID ||:

        cat $outputdir/basic/results-rate-${index}.bin | vegeta report -reporter text > $outputdir/basic/report-r${index}-text.txt
        cat $outputdir/basic/results-rate-${index}.bin | vegeta report -reporter plot > $outputdir/basic/report-r${index}-plot.html
        cat $outputdir/basic/results-rate-${index}.bin | vegeta report -reporter='hist[0,2ms,4ms,6ms,8ms,10ms,20ms,30ms,40ms,60ms,80ms,100ms,200ms,400ms,800ms,1600ms,3200ms,6400ms]' > $outputdir/basic/report-r${index}-hist.txt
    fi

    cat  $outputdir/token/report-r${index}-text.txt  $outputdir/basic/report-r${index}-text.txt ||:
done


