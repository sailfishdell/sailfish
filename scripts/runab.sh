#!/bin/sh

set -e

unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy

scriptdir=$(cd $(dirname $0); pwd)
. ${scriptdir}/common-vars.sh

outputdir=${1:-out/}
uri=${uri:-/redfish/v1/Managers/CMC.Integrated.1}
runtoken=${runtoken:-1}
runbasic=${runbasic:-0}

if [ -z "${outputdir}" ]; then
    echo Error: Need to set output directory
    exit 1
fi

rm -rf ${outputdir}
mkdir -p $outputdir

set_auth_header

echo "Running ab"
numreqs=${numreqs:-1000}
timelimit=10

for i in $(seq 30 ) $(seq 40 10 100) ; do
    index=$(printf "%03d" $i)
    outfile=${outputdir}/results-c${index}-r${numreqs}

    if [ "${runtoken}" = "1" ]; then
        echo ab -t ${timelimit} -c $i -n ${numreqs} -k -g ${outfile}-token.plot -e ${outfile}-token.csv  -H \"$AUTH_HEADER\" -H \"content-type: application/json\" ${BASE}${uri} > ${outfile}-CLI.sh
        ab -t ${timelimit} -c $i -n ${numreqs} -k -g ${outfile}-token.plot -e ${outfile}-token.csv  -H "$AUTH_HEADER" -H "content-type: application/json" ${BASE}${uri} | tee ${outfile}-token.txt
    fi
    sleep 1
    if [ "${runbasic}" = "1" ]; then
        ab -t ${timelimit} -c $i -n ${numreqs} -k -g ${outfile}-basic.plot -e ${outfile}-basic.csv -A ${user}:${pass}  -H "content-type: application/json" ${BASE}${uri} | tee ${outfile}-basic.txt
    fi
    sleep 1
done
