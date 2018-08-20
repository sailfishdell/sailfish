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
#   mount -o remount,exec /mnt/persistent_data/
#   setenforce 0
#
# back on your dev machine, run:
#   ssh-copy-id root@IP.ADD.RESS
#
# turn off ssh session timeouts so that sailfish doesn't get killed in the middle of a benchmark
#   racadm config -g cfgSessionManagement -o cfgSsnMgtSshIdleTimeout 0
#
# set up build environment:
#   mkdir -p ~/go/src/github.com/superchalupa
#   ln -s ~/14g/externalsrc/go-redfish  ~/go/src/github.com/superchalupa/go-redfish
#
# compile sailfish
#   GOARCH=arm GOARM=5 go build  github.com/superchalupa/go-redfish/cmd/sailfish
#
# copy to idrac:
#   scp ./sailfish ./redfish-logging.yaml ./redfish.yaml  root@10.255.3.54:/flash/data0/
#
# run sailfish: ssh root@IP
#   cd /flash/data0
#   ./sailfish

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
export runbasic=${runbasic:-0}

# run token auth tests for each that supports
export runtoken=${runtoken:-1}
mkdir ${out} ||:

LOGFILE=$out/script-output.txt
exec 1> >(exec -a 'LOGGING TEE' tee $LOGFILE) 2>&1
TEEPID=$!

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



###########################
## odatalite tests
###########################
export user=root
export pass=calvin
export uri=/redfish/v1/Managers/iDRAC.Embedded.1

export rps="$(seq 1 2 50) $(seq 50 10 120)"

host=$IDRACHOST port=443 ${scriptdir}/walk.sh        ${out}/odatalite-walk

# this uri is using the new sqlite
uri=${sqliteuri}    \
host=$IDRACHOST port=443 ${scriptdir}/runab.sh       ${out}/sqlite-ab

# select out the new sqlite URIs for specific bench by vegeta
mkdir -p ${out}/sqlite-vegeta ||:
cat ${out}/odatalite-walk/errors.txt ${out}/odatalite-walk/to-visit.txt | sort | uniq -u | grep -v /redfish/v1/SessionService/Sessions/ | grep PCIeDevice  > ${out}/sqlite-vegeta/to-visit.txt  ||:
host=$IDRACHOST port=443 ${scriptdir}/runvegeta.sh ${out}/sqlite-vegeta

# select out the new CIM URIs for specific bench by vegeta
mkdir -p ${out}/cim-vegeta ||:
cat ${out}/odatalite-walk/errors.txt ${out}/odatalite-walk/to-visit.txt | sort | uniq -u | grep -v /redfish/v1/SessionService/Sessions/ | grep /redfish/v1/Dell  > ${out}/cim-vegeta/to-visit.txt  ||:
host=$IDRACHOST port=443 ${scriptdir}/runvegeta.sh ${out}/cim-vegeta

# re-use walk data for vegeta
mkdir -p ${out}/odatalite-walk ||:
cat ${out}/odatalite-walk/errors.txt ${out}/odatalite-walk/to-visit.txt | sort | uniq -u | grep -v /redfish/v1/SessionService/Sessions/  > ${out}/odatalite-vegeta/to-visit.txt  ||:
host=$IDRACHOST port=443 ${scriptdir}/runvegeta.sh ${out}/odatalite-vegeta
host=$IDRACHOST port=443 ${scriptdir}/runab.sh       ${out}/odatalite-ab

##########################
# END
##########################

# close FDs to ensure tee finishes
exec 1>&0 2>&1
if [ -n "$logging_tee_pid" ];then
    kill $logging_tee_pid
fi

