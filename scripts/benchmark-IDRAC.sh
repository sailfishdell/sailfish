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

export CURL_OPTS=-k
export prot=https
export sqliteuri=${sqliteuri:-/redfish/v1/Chassis/System.Embedded.1/PCIeDevice/3-0}

# run TOP and save results during each run
export profile=${profile:-1}

# run basic auth tests for each that supports
export runbasic=${runbasic:-1}

# run token auth tests for each that supports
export runtoken=${runtoken:-1}
mkdir bench ||:

####################
# odatalite tests
####################
export user=root
export pass=calvin

export uri=/redfish/v1/Managers/iDRAC.Embedded.1
host=$IDRACHOST port=443 ${scriptdir}/walk.sh        ${out}/odatalite-walk
host=$IDRACHOST port=443 ${scriptdir}/runab.sh       ${out}/odatalite-OLD-ab
host=$IDRACHOST port=443 ${scriptdir}/vegeta-test.sh ${out}/odatalite-ALL-vegeta

# this uri is using the new sqlite
uri=${sqliteuri}    \
host=$IDRACHOST port=443 ${scriptdir}/runab.sh       ${out}/odatalite-NEW-ab

# select out the new sqlite URIs for specific bench by vegeta
mkdir ${out}/odatalite-NEW-vegeta ||:
grep PCIeDevice ${out}/odatalite-ALL-vegeta/to-visit.txt > ${out}/odatalite-NEW-vegeta/to-visit.txt
host=$IDRACHOST port=443 ${scriptdir}/vegeta-test.sh ${out}/odatalite-NEW-vegeta


####################
# go-redfish tests
####################
export user=Administrator
export pass=password
host=$IDRACHOST TOKEN= port=8443 ${scriptdir}/walk.sh        ${out}/go-https-walk
host=$IDRACHOST TOKEN= port=8443 ${scriptdir}/vegeta-test.sh ${out}/go-https-vegeta
host=$IDRACHOST TOKEN= port=8443 ${scriptdir}/runhey.sh      ${out}/go-https-hey
host=$IDRACHOST TOKEN= port=8443 ${scriptdir}/runab.sh       ${out}/go-https-ab


# test go-redfish through apache
mkdir -p ${out}/go-apache-vegeta
grep /Attributes ${out}/go-https-vegeta/to-visit.txt > ${out}/go-apache-vegeta/to-visit.txt
host=$IDRACHOST port=2443 ${scriptdir}/vegeta-test.sh     ${out}/go-apache-vegeta
host=$IDRACHOST port=2443 ${scriptdir}/runhey.sh          ${out}/go-apache-hey
host=$IDRACHOST port=2443 ${scriptdir}/runab.sh           ${out}/go-apache-ab


