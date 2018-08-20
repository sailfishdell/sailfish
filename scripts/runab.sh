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

rm -rf $outputdir/{token,basic} ||:
mkdir -p $outputdir $outputdir/token $outputdir/basic
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
single_step_rps_start=1
single_step_rps_end=30
ten_step_rps_start=40
ten_step_rps_end=200
rps=${rps:-"$(seq $single_step_rps_start $single_step_rps_end) $(seq $ten_step_rps_start 10 $ten_step_rps_end)"}

# to save CPU TOP information, you have to have SSH access to the box and already copied your ssh key. use "ssh-copy-id root@IP" to copy your key.
# on idrac, you also have to disable the avocent NSS module by mount binding /etc/nsswitch.conf and removing avct from passwd line
savetop() {
    echo "Starting TOP for apachebench run. RATE: $index" > $1
    ssh root@${host} 'top -b -d1 -o %CPU' >> $1 &
    SSHPID=$!
}

for i in ${rps} ; do
    index=$(printf "%03d" $i)

    if [ "${runtoken}" = "1" ]; then
        outfile=${outputdir}/token/results-c${index}-r${numreqs}
        [ "${profile}" == 1 ] && savetop ${outfile}-CPU.txt
        ab -t ${timelimit} -c $i -n ${numreqs} -k -g ${outfile}.plot -e ${outfile}.csv  -H "$AUTH_HEADER" -H "content-type: application/json" ${BASE}${uri} | tee ${outfile}.txt
        [ -n "$SSHPID" ] && kill $SSHPID ||:
    fi
    sleep 1
    if [ "${runbasic}" = "1" ]; then
        outfile=${outputdir}/basic/results-c${index}-r${numreqs}
        [ "${profile}" == 1 ] && savetop ${outfile}-CPU.txt
        ab -t ${timelimit} -c $i -n ${numreqs} -k -g ${outfile}.plot -e ${outfile}.csv -A ${user}:${pass}  -H "content-type: application/json" ${BASE}${uri} | tee ${outfile}.txt
        [ -n "$SSHPID" ] && kill $SSHPID ||:
    fi
    sleep 1
done

for i in {token,basic}; do
    [ -d ${outputdir}/$i ] || continue
    grep ^Total: ${outputdir}/${i}/results-c*-r1000.txt   > ${outputdir}/LATENCIES-${i}.txt ||:
    grep ^Request ${outputdir}/${i}/results-c*-r1000.txt  > ${outputdir}/RPS-${i}.txt ||:

    # CPU 'top' measurements. The first and last measurement aren't full
    # measurements, so the stuff below throws out first and last measurement
    for j in $(find ${outputdir}/${i} -name *-CPU.txt | sort); do
        grep ^%Cpu $j | head -n-1 | tail -n+2 | perl -p -i -e "s#^#${j}: #";
    done  > ${outputdir}/TOTALCPU-${i}.txt ||:


    ##################
    # CSV and PLOT
    ##################

    # CPU 'top' measurements. The first and last measurement aren't full
    # measurements, so the stuff below throws out first and last measurement
    # then it averages all the measurements for each concurrency level
    for j in $(find ${outputdir}/${i} -name *-CPU.txt | sort); do
        concurrent=$(basename ${j}  | sort | cut -d- -f2 | perl -p -i -e 's/^c//; s/^0+//;')

        sum=0
        count=0
        average=$(grep ^%Cpu $j | head -n-1 | tail -n+2 |
        while read line
        do
            idle=$(echo $line | cut -d, -f4 | awk '{print $1}')
            sum=$( echo $idle + $sum | bc -l)
            count=$(( count + 1 ))
            echo "100 - ( $sum / $count )" | bc -l
        done | tail -n1 )
        echo $concurrent, $average

    done  > ${outputdir}/TOTALCPU-${i}.csv ||:

    cat $scriptdir/plot/cpu.plot | perl -p -i -e "s#BASE#${outputdir}/TOTALCPU-${i}#g;" > ${outputdir}/TOTALCPU-${i}.plot
    gnuplot ${outputdir}/TOTALCPU-${i}.plot ||:

    ##
    # graph requests per second
    ##
    >  ${outputdir}/RPS-${i}.csv
    for j in $(find ${outputdir}/${i} -name results-*.txt | grep -v CPU | sort); do
        concurrent=$(basename ${j}  | sort | cut -d- -f2 | perl -p -i -e 's/^c//; s/^0+//;')
        RPS=$(cat $j | grep ^Requests | cut -d: -f2 | awk '{print $1}')
        echo "$concurrent, $RPS" >> ${outputdir}/RPS-${i}.csv
    done
    cat $scriptdir/plot/ab-rps.plot | perl -p -i -e "s#BASE#${outputdir}/RPS-${i}#g;" > ${outputdir}/RPS-${i}.plot
    gnuplot ${outputdir}/RPS-${i}.plot ||:

    >  ${outputdir}/LATENCIES-${i}.csv
    for j in $(find ${outputdir}/${i} -name results-*.txt | grep -v CPU | sort); do
        concurrent=$(basename ${j}  | sort | cut -d- -f2 | perl -p -i -e 's/^c//; s/^0+//;')
        MIN=$(cat $j | grep ^Total: | cut -d: -f2 | awk '{print $1}')
        MEAN=$(cat $j | grep ^Total: | cut -d: -f2 | awk '{print $2}')
        MEDIAN=$(cat $j | grep ^Total: | cut -d: -f2 | awk '{print $4}')
        MAX=$(cat $j | grep ^Total: | cut -d: -f2 | awk '{print $5}')
        echo "$concurrent, $MIN, $MEAN, $MEDIAN, $MAX" >> ${outputdir}/LATENCIES-${i}.csv
    done
    cat $scriptdir/plot/ab-lats.plot | perl -p -i -e "s#BASE#${outputdir}/LATENCIES-${i}#g;" > ${outputdir}/LATENCIES-${i}.plot
    gnuplot ${outputdir}/LATENCIES-${i}.plot ||:

done


# close FDs to ensure tee finishes
exec 1>&0 2>&1
if [ -n "$logging_tee_pid" ];then
    kill $logging_tee_pid
fi

