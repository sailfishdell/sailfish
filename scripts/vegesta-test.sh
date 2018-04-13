#!/bin/sh

set -e

unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy

scriptdir=$(cd $(dirname $0); pwd)
skiplist=${2:-}

tempdir=$(mktemp -d ./output-XXXXXX)
trap 'rm -rf $tempdir' EXIT

. $scriptdir/walk.sh $tempdir

# reset outputdir because walk stomps
outputdir=${1:-out/}

user=${user:-Administrator}
pass=${pass:-password}
host=${host:-localhost}
port=${port:-8443}

if [ "${host}" = "localhost" ]; then
    cacert=${cacert:-./ca.crt}
else
    cacert=${cacert:-./${host}-ca.crt}
fi

#eval $(scripts/login.sh $user $pass)

host=${host:-localhost}
if [ "${port}" = "443" -o "${port}" = "8443" ]; then
    prot=${prot:-https}
else
    prot=${prot:-http}
fi
BASE=${prot}://${host}:${port}

AUTH_HEADER=${AUTH_HEADER:-}
if [ -n "$TOKEN" ]; then
    AUTH_HEADER="Authorization: Bearer $TOKEN"
fi


echo "Running vegeta"

time=10s
mkdir -p $outputdir $outputdir/token $outputdir/basic
cat $tempdir/to-visit.txt | perl -p -i -e "s|^|GET ${BASE}|" > $outputdir/vegeta-targets.txt
cat $tempdir/to-visit.txt | perl -p -i -e "s|^|GET ${prot}://${user}:${pass}\@${host}:${port}|" > $outputdir/basic/vegeta-targets.txt

for i in $(seq 10 10 1000) ; do 
    index=$(printf "%03d" $i)
    vegeta attack -targets $outputdir/vegeta-targets.txt -output $outputdir/token/results-rate-${index}.bin -header "$AUTH_HEADER" -duration=${time} -root-certs $cacert -rate $i

    cat $outputdir/token/results-rate-${index}.bin | vegeta report -reporter text > $outputdir/token/report-r${index}-text.txt
    cat $outputdir/token/results-rate-${index}.bin | vegeta report -reporter plot > $outputdir/token/report-r${index}-plot.html
    cat $outputdir/token/results-rate-${index}.bin | vegeta report -reporter='hist[0,2ms,4ms,6ms,8ms,10ms,20ms,30ms,40ms,60ms,80ms,100ms]' > $outputdir/token/report-r${index}-hist.txt

    vegeta attack -targets $outputdir/basic/vegeta-targets.txt -output $outputdir/basic/results-rate-${index}.bin -duration=${time} -root-certs $cacert -rate $i
    cat $outputdir/basic/results-rate-${index}.bin | vegeta report -reporter text > $outputdir/basic/report-r${index}-text.txt
    cat $outputdir/basic/results-rate-${index}.bin | vegeta report -reporter plot > $outputdir/basic/report-r${index}-plot.html
    cat $outputdir/basic/results-rate-${index}.bin | vegeta report -reporter='hist[0,2ms,4ms,6ms,8ms,10ms,20ms,30ms,40ms,60ms,80ms,100ms]' > $outputdir/basic/report-r${index}-hist.txt

    cat  $outputdir/token/report-r${index}-text.txt  $outputdir/basic/report-r${index}-text.txt
done


