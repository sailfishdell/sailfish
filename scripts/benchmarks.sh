#!/bin/bash

set -e
set -x

scriptdir=$(cd $(dirname $0); pwd)

if [ -z "${ECHOST}" -o -z "${IDRACHOST}" ]; then
    echo "need to set host variables"
    exit 1
fi

# FOR EC ODATALITE TESTING ONLY: set TOKEN to 'oauthtest token' output and this script will set and unset as appropriate to benchark each stack

export CURL_OPTS=-k
export prot=https

# run TOP and save results during each run
export profile=1

# run basic auth tests for each that supports
export runbasic=1

# run token auth tests for each that supports
export runtoken=1

####################
# go-redfish tests
####################
export user=Administrator
export pass=password
host=$ECHOST TOKEN= port=8443 ${scriptdir}/walk.sh        bench-go-https-walk
host=$ECHOST TOKEN= port=8443 ${scriptdir}/runab.sh       bench-go-https-ab
host=$ECHOST TOKEN= port=8443 ${scriptdir}/vegeta-test.sh bench-go-https-vegeta


####################
# odatalite tests
####################
export user=root
export pass=calvin
# used by ab:
export uri=/redfish/v1/Managers/iDRAC.Embedded.1
host=$IDRACHOST port=443 ${scriptdir}/walk.sh        bench-odatalite-walk
host=$IDRACHOST port=443 ${scriptdir}/runab.sh       bench-odatalite-OLD-ab
host=$IDRACHOST port=443 ${scriptdir}/vegeta-test.sh bench-odatalite-vegeta

# this uri is using the new sqlite
export uri=/redfish/v1/Chassis/System.Embedded.1/PCIeDevice/94-0 
host=$IDRACHOST port=443 ${scriptdir}/runab.sh       bench-odatalite-NEW-ab

# select out the new sqlite URIs for specific bench by vegeta
mkdir bench-odatalite-NEW-vegeta ||:
grep PCIeDevice bench-odatalite-vegeta/to-visit.txt > bench-odatalite-NEW-vegeta/to-visit.txt
host=$IDRACHOST port=443 ${scriptdir}/vegeta-test.sh bench-odatalite-NEW-vegeta


# extract timings
for i in bench-go-https-walk bench-odatalite-walk; do
    grep ^Total ${i}/script-output.txt  | sort | grep PIPELINE > ${i}/WALK-TIMING-pipelined.txt
    grep ^Total ${i}/script-output.txt  | sort | grep -v PIPELINE > ${i}/WALK-TIMING-individual.txt
done

for i in bench-go-https-ab bench-odatalite-OLD-ab bench-odatalite-NEW-ab ; do
    grep ^Total: ${i}/results-c*-r1000-token.txt  > ${i}/LATENCIES-token.txt
    grep ^Total: ${i}/results-c*-r1000-basic.txt  > ${i}/LATENCIES-basic.txt
    grep ^Request ${i}/results-c*-r1000-token.txt  > ${i}/RPS-token.txt
    grep ^Request ${i}/results-c*-r1000-basic.txt  > ${i}/RPS-basic.txt
done

for i in bench-go-https-vegeta bench-odatalite-vegeta  bench-odatalite-NEW-vegeta; do
    grep ^Latencies ${i}/basic/report-r*-text.txt > ${i}/LATENCIES-basic.txt
    grep ^Latencies ${i}/token/report-r*-text.txt > ${i}/LATENCIES-token.txt
    grep ^Success ${i}/basic/report-r*-text.txt > ${i}/SUCCESSRATE-basic.txt
    grep ^Success ${i}/token/report-r*-text.txt > ${i}/SUCCESSRATE-token.txt
done
