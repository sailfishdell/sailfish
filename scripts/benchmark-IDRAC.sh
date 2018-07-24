#!/bin/bash

set -e
set -x

# PREPARATION
# get root ssh access to the system:
#   mount --bind /flash/data0/home/root/ /home/root
#   cp /etc/nsswitch.conf /tmp
#   perl -p -i -e 's/avct//g;' /tmp/nsswitch.conf
#   mount --bind /tmp/nsswitch.conf /etc/nsswitch.conf
#   restorecon /etc/nsswitch.conf
#   ssh-copy-id root@IP.ADD.RESS


scriptdir=$(cd $(dirname $0); pwd)
out=${out:-bench}
[ -e ${out}/config ] && . ${out}/config ||:

if [ -z "${GORFHOST}" -a -z "${IDRACHOST}" ]; then
    echo "need to set host variables"
    exit 1
fi

# FOR EC ODATALITE TESTING ONLY: set TOKEN to 'oauthtest token' output and this script will set and unset as appropriate to benchark each stack

export CURL_OPTS=-k
export prot=https
export sqliteuri=${sqliteuri:-/redfish/v1/Chassis/System.Embedded.1/PCIeDevice/3-0}

# run TOP and save results during each run
export profile=1

# run basic auth tests for each that supports
export runbasic=1

# run token auth tests for each that supports
export runtoken=1
mkdir bench ||:

####################
# odatalite tests
####################
export user=root
export pass=calvin

export uri=/redfish/v1/Managers/iDRAC.Embedded.1
host=$IDRACHOST port=443 ${scriptdir}/walk.sh        ${out}/odatalite-walk
host=$IDRACHOST port=443 ${scriptdir}/runab.sh       ${out}/odatalite-OLD-ab
host=$IDRACHOST port=443 ${scriptdir}/vegeta-test.sh ${out}/odatalite-vegeta

# this uri is using the new sqlite
uri=${sqliteuri}    \
host=$IDRACHOST port=443 ${scriptdir}/runab.sh       ${out}/odatalite-NEW-ab

# select out the new sqlite URIs for specific bench by vegeta
mkdir ${out}/odatalite-NEW-vegeta ||:
grep PCIeDevice ${out}/odatalite-vegeta/to-visit.txt > ${out}/odatalite-NEW-vegeta/to-visit.txt
host=$IDRACHOST port=443 ${scriptdir}/vegeta-test.sh ${out}/odatalite-NEW-vegeta
