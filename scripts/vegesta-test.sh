#!/bin/sh

set -e

unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy

scriptdir=$(cd $(dirname $0); pwd)
. ${scriptdir}/common-vars.sh

outputdir=${1:-out/}
skiplist=${2:-}


if [ ! -e $outputdir/to-visit.txt ]; then
    tempdir=$(mktemp -d ./output-XXXXXX)
    trap 'rm -rf $tempdir' EXIT

    . $scriptdir/walk.sh $tempdir
    outputdir=${1:-out/}

    mkdir -p $outputdir/
    cp $tempdir/to-visit.txt $outputdir/to-visit.txt

    rm -rf $tempdir
fi

rm -rf $outputdir/{token,basic}
mkdir -p $outputdir $outputdir/token $outputdir/basic
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

for i in $(seq 30 ) $(seq 40 10 100) ; do
    index=$(printf "%03d" $i)
    vegeta attack -targets $outputdir/vegeta-targets.txt -output $outputdir/token/results-rate-${index}.bin -header "$AUTH_HEADER" -duration=${time} $cert_opt -rate $i

    cat $outputdir/token/results-rate-${index}.bin | vegeta report -reporter text > $outputdir/token/report-r${index}-text.txt
    cat $outputdir/token/results-rate-${index}.bin | vegeta report -reporter plot > $outputdir/token/report-r${index}-plot.html
    cat $outputdir/token/results-rate-${index}.bin | vegeta report -reporter='hist[0,2ms,4ms,6ms,8ms,10ms,20ms,30ms,40ms,60ms,80ms,100ms,200ms,400ms,800ms,1600ms,3200ms,6400ms]' > $outputdir/token/report-r${index}-hist.txt

    vegeta attack -targets $outputdir/basic/vegeta-targets.txt -output $outputdir/basic/results-rate-${index}.bin -duration=${time} $cert_opt -rate $i
    cat $outputdir/basic/results-rate-${index}.bin | vegeta report -reporter text > $outputdir/basic/report-r${index}-text.txt
    cat $outputdir/basic/results-rate-${index}.bin | vegeta report -reporter plot > $outputdir/basic/report-r${index}-plot.html
    cat $outputdir/basic/results-rate-${index}.bin | vegeta report -reporter='hist[0,2ms,4ms,6ms,8ms,10ms,20ms,30ms,40ms,60ms,80ms,100ms,200ms,400ms,800ms,1600ms,3200ms,6400ms]' > $outputdir/basic/report-r${index}-hist.txt

    cat  $outputdir/token/report-r${index}-text.txt  $outputdir/basic/report-r${index}-text.txt
done


