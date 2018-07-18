#!/bin/bash

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
LOGFILE=$outputdir/script-output.txt
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

set_auth_header

echo "Running ab"
numreqs=${numreqs:-1000}
timelimit=${timelimit:-10}


# to save CPU TOP information, you have to have SSH access to the box and already copied your ssh key. use "ssh-copy-id root@IP" to copy your key.
# on idrac, you also have to disable the avocent NSS module by mount binding /etc/nsswitch.conf and removing avct from passwd line
savetop() {
    echo "Starting TOP for apachebench run. RATE: $index" > $1
    ssh root@${host} 'top -b -d1 -o %CPU' >> $1 &
    SSHPID=$!
}

for i in $(seq 30 ) $(seq 40 10 120) ; do
    index=$(printf "%03d" $i)
    outfile=${outputdir}/results-c${index}-r${numreqs}

    if [ "${runtoken}" = "1" ]; then
        [ "${profile}" == 1 ] && savetop ${outfile}-token-CPU.txt
        ab -t ${timelimit} -c $i -n ${numreqs} -k -g ${outfile}-token.plot -e ${outfile}-token.csv  -H "$AUTH_HEADER" -H "content-type: application/json" ${BASE}${uri} | tee ${outfile}-token.txt
        [ -n "$SSHPID" ] && kill $SSHPID ||:
    fi
    sleep 1
    if [ "${runbasic}" = "1" ]; then
        [ "${profile}" == 1 ] && savetop ${outfile}-basic-CPU.txt
        ab -t ${timelimit} -c $i -n ${numreqs} -k -g ${outfile}-basic.plot -e ${outfile}-basic.csv -A ${user}:${pass}  -H "content-type: application/json" ${BASE}${uri} | tee ${outfile}-basic.txt
        [ -n "$SSHPID" ] && kill $SSHPID ||:
    fi
    sleep 1
done

# close FDs to ensure tee finishes
exec 1>&0 2>&1
if [ -n "$logging_tee_pid" ];then
    while ps --pid $logging_tee_pid > /dev/null 2>&1
    do
        sleep 1
    done
fi

