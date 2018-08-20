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
# go-redfish behind APACHE
##########################
export user=root
export pass=calvin
export uri=/redfish/v1/Managers/CMC.Integrated.1

# sailfish behind apache goes over 100
export rps="$(seq 1 2 50) $(seq 50 10 120)"

# we are using apache, so we need a valid token for apache fcgi auth. manually set it
# because this token expires, lets refresh for every bench
eval $(CURL_OPTS=-k host=$IDRACHOST port=443  user=$user pass=$pass ./scripts/login.sh root calvin)

prot=https host=$IDRACHOST port=2443 ${scriptdir}/walk.sh        ${out}/sailfish-apache-walk
mkdir -p ${out}/sailfish-apache-vegeta ||:
cat ${out}/sailfish-apache-walk/errors.txt ${out}/sailfish-apache-walk/to-visit.txt | sort | uniq -u | grep -v /redfish/v1/SessionService/Sessions/  > ${out}/sailfish-apache-vegeta/to-visit.txt  ||:

eval $(CURL_OPTS=-k host=$IDRACHOST port=443  user=root pass=calvin ./scripts/login.sh root calvin)
prot=https host=$IDRACHOST port=2443 ${scriptdir}/runvegeta.sh   ${out}/sailfish-apache-vegeta

eval $(CURL_OPTS=-k host=$IDRACHOST port=443  user=root pass=calvin ./scripts/login.sh root calvin)
prot=https host=$IDRACHOST port=2443 ${scriptdir}/runhey.sh      ${out}/sailfish-apache-hey

# ab doesn't do anything interesting above 100 concurrent
export rps="$(seq 1 2 50) $(seq 50 10 100)"
eval $(CURL_OPTS=-k host=$IDRACHOST port=443  user=root pass=calvin ./scripts/login.sh root calvin)
prot=https host=$IDRACHOST port=2443 ${scriptdir}/runab.sh       ${out}/sailfish-apache-ab

unset X_AUTH_TOKEN
unset AUTH_HEADER
unset SESSION_URI


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

export rps="$(seq 1 2 50) $(seq 50 10 150)"
host=$IDRACHOST port=8443 ${scriptdir}/runhey.sh    ${out}/sailfish-https-hey

# ab doesn't do anything interesting above 100 concurrent
export rps="$(seq 1 2 50) $(seq 50 10 100)"
host=$IDRACHOST port=8443 ${scriptdir}/runab.sh     ${out}/sailfish-https-ab

##########################
# END
##########################
mkdir -p ${out}/plot ||:
cp -a ${scriptdir}/plot/*.plot ${out}/plot/.
(cd ${out}; cat plot/compare*plot | gnuplot)

##########################
# END
##########################

# close FDs to ensure tee finishes
exec 1>&0 2>&1
if [ -n "$logging_tee_pid" ];then
    kill $logging_tee_pid
fi

