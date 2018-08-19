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

##########################
# go-redfish tests
##########################
export user=Administrator
export pass=password

# sailfish goes over 100
export rps="$(seq 1 2 50) $(seq 50 10 250)"

# running EC go redfish stack, so test ec uri
export uri=/redfish/v1/Managers/CMC.Integrated.1
host=$IDRACHOST port=8443 ${scriptdir}/walk.sh      ${out}/sailfish-https-walk
mkdir -p ${out}/sailfish-https-vegeta ||:
cat ${out}/sailfish-https-walk/errors.txt ${out}/sailfish-https-walk/to-visit.txt | sort | uniq -u | grep -v /redfish/v1/SessionService/Sessions/  > ${out}/sailfish-https-vegeta/to-visit.txt  ||:
host=$IDRACHOST port=8443 ${scriptdir}/runvegeta.sh ${out}/sailfish-https-vegeta
host=$IDRACHOST port=8443 ${scriptdir}/runhey.sh    ${out}/sailfish-https-hey
host=$IDRACHOST port=8443 ${scriptdir}/runab.sh     ${out}/sailfish-https-ab

##########################
# END
##########################

# close FDs to ensure tee finishes
exec 1>&0 2>&1
if [ -n "$logging_tee_pid" ];then
    kill $logging_tee_pid
fi

