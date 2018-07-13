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
mkdir -p $outputdir
cat $outputdir/to-visit.txt | head -n 2 | perl -p -i -e "s|^|${BASE}|" > $outputdir/siege-targets.txt
# cat $outputdir/to-visit.txt | perl -p -i -e "s|^|${prot}://${user}:${pass}\@${host}:${port}|" > $outputdir/basic/siege-targets.txt

set_auth_header

echo "Running siege"

time=10s

#for i in $(seq 30 ) $(seq 40 10 100) ; do
#    index=$(printf "%03d" $i)

    siege -R ${scriptdir}/siege.conf -c $i -r 1 -H "$AUTH_HEADER" -f $outputdir/siege-targets.txt

    #siege attack -targets $outputdir/siege-targets.txt -output $outputdir/token/results-rate-${index}.bin -header "$AUTH_HEADER" -duration=${time} $cert_opt -rate $i
    #siege attack -targets $outputdir/basic/siege-targets.txt -output $outputdir/basic/results-rate-${index}.bin -duration=${time} $cert_opt -rate $i
#done


