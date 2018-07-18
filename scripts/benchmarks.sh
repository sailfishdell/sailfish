#!/bin/sh

set -e
set -x

scriptdir=$(cd $(dirname $0); pwd)

if [ -z "${ECHOST}" -o -z "${IDRACHOST}" ]; then
    echo "need to set host variables"
    exit 1
fi

# set TOKEN to 'oauthtest token' output and this script will set and unset as appropriate to benchark each stack

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
host=$IDRACHOST port=443 ${scriptdir}/walk.sh        bench-odatalite-walk
host=$IDRACHOST port=443 ${scriptdir}/runab.sh       bench-odatalite-OLD-ab
host=$IDRACHOST port=443 ${scriptdir}/vegeta-test.sh bench-odatalite-vegeta

# this uri is using the new sqlite
export uri=/redfish/v1/Chassis/System.Embedded.1/PCIeDevice/94-0 
host=$IDRACHOST port=443 ${scriptdir}/runab.sh       bench-odatalite-NEW-ab
