#!/bin/bash

set -x
set -e

unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy

scriptdir=$(cd $(dirname $0); pwd)
. ${scriptdir}/common-vars.sh

outputdir=${1:-out/}
skiplist=${2:-}
runtoken=${runtoken:-1}
runbasic=${runbasic:-0}

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

if [ ! -e $outputdir/to-visit.txt ]; then
    tempdir=$(mktemp -d ./output-XXXXXX)
    trap 'cleanup; [ -n "$tepdir" ] && rm -rf $tempdir' EXIT

    . $scriptdir/walk.sh $tempdir
    outputdir=${1:-out/}

    mkdir -p $outputdir/
    cat $tempdir/errors.txt $tempdir/to-visit.txt | sort | uniq -u | grep -v /redfish/v1/SessionService/Sessions/  > $outputdir/to-visit.txt  ||:

    rm -rf $tempdir
fi

rm -rf $outputdir/{token,basic} ||:
mkdir -p $outputdir $outputdir/token $outputdir/basic
LOGFILE=$outputdir/script-output.txt
exec 1> >(exec -a 'LOGGING TEE' tee $LOGFILE) 2>&1
TEEPID=$!

cat $outputdir/to-visit.txt | perl -p -i -e "s|^|GET ${BASE}|" > $outputdir/vegeta-targets.txt
cat $outputdir/to-visit.txt | perl -p -i -e "s|^|GET ${prot}://${user}:${pass}\@${host}:${port}|" > $outputdir/basic/vegeta-targets.txt

set_auth_header

if [ -n "${cacert_file}" ] ;then
    cert_opt="-root-certs $cacert_file"
else
    cert_opt="-insecure"
fi

echo "Running vegeta"

time=10s
single_step_rps_start=1
single_step_rps_end=30
ten_step_rps_start=40
ten_step_rps_end=200
rps=${rps:-"$(seq $single_step_rps_start $single_step_rps_end) $(seq $ten_step_rps_start 10 $ten_step_rps_end)"}

savetop() {
    echo "Starting TOP for vegeta run. RATE: $index" > $1
    ssh root@${host} 'top -b -d1 -o %CPU' >> $1 &
    SSHPID=$!
}

for i in $rps ; do
    index=$(printf "%03d" $i)

    if [ ${runtoken} == 1 ]; then
        [ "${profile}" == 1 ] && savetop $outputdir/token/report-r${index}-CPU.txt
        vegeta attack ${VEGETA_OPTS} -targets $outputdir/vegeta-targets.txt -output $outputdir/token/results-rate-${index}.bin -header "$AUTH_HEADER" -duration=${time} $cert_opt -rate $i
        [ -n "$SSHPID" ] && kill $SSHPID ||:

        cat $outputdir/token/results-rate-${index}.bin | vegeta report -reporter text > $outputdir/token/report-r${index}-text.txt
        cat $outputdir/token/results-rate-${index}.bin | vegeta report -reporter json > $outputdir/token/report-r${index}.json
        cat $outputdir/token/results-rate-${index}.bin | vegeta report -reporter plot > $outputdir/token/report-r${index}-plot.html
        cat $outputdir/token/results-rate-${index}.bin | vegeta report -reporter='hist[0,2ms,4ms,6ms,8ms,10ms,20ms,30ms,40ms,60ms,80ms,100ms,200ms,400ms,800ms,1600ms,3200ms,6400ms]' > $outputdir/token/report-r${index}-hist.txt
    fi

    if [ ${runbasic} == 1 ]; then
        [ "${profile}" == 1 ] && savetop $outputdir/basic/report-r${index}-CPU.txt
        vegeta attack ${VEGETA_OPTS} -targets $outputdir/basic/vegeta-targets.txt -output $outputdir/basic/results-rate-${index}.bin -duration=${time} $cert_opt -rate $i
        [ -n "$SSHPID" ] && kill $SSHPID ||:

        cat $outputdir/basic/results-rate-${index}.bin | vegeta report -reporter text > $outputdir/basic/report-r${index}-text.txt
        cat $outputdir/basic/results-rate-${index}.bin | vegeta report -reporter json > $outputdir/basic/report-r${index}.json
        cat $outputdir/basic/results-rate-${index}.bin | vegeta report -reporter plot > $outputdir/basic/report-r${index}-plot.html
        cat $outputdir/basic/results-rate-${index}.bin | vegeta report -reporter='hist[0,2ms,4ms,6ms,8ms,10ms,20ms,30ms,40ms,60ms,80ms,100ms,200ms,400ms,800ms,1600ms,3200ms,6400ms]' > $outputdir/basic/report-r${index}-hist.txt
    fi

    cat  $outputdir/*/report-r${index}-text.txt ||:
done

processdirs=
if [ ${runbasic} -eq 1 ]; then
    processdirs="$processdirs basic"
fi
if [ ${runtoken} -eq 1 ]; then
    processdirs="$processdirs token"
fi
for i in $processdirs; do
    [ -d ${outputdir}/$i ] || continue
    grep ^Latencies ${outputdir}/${i}/report-r*-text.txt > ${outputdir}/LATENCIES-${i}.txt ||:
    grep ^Success ${outputdir}/${i}/report-r*-text.txt > ${outputdir}/SUCCESSRATE-${i}.txt ||:

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
        concurrent=$(basename ${j}  | sort | cut -d- -f2 | perl -p -i -e 's/^r//; s/^0+//;')

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

    cat $scriptdir/plot/cpu.plot | perl -p -i -e "s#BASE#${outputdir}/TOTALCPU-${i}#g;" > ${outputdir}/cpu.plot
    gnuplot ${outputdir}/cpu.plot ||:


    > ${outputdir}/LATENCIES-${i}.csv
    for j in $(find ${outputdir}/${i} -name report*.json | sort); do
        concurrent=$(basename ${j} .json  | sort | cut -d- -f2 | perl -p -i -e 's/^r//; s/^0+//;')
        MEAN=$(cat $j | jq '."latencies"."mean" / 1000000')
        P50=$(cat $j | jq '."latencies"."50th" / 1000000')
        P95=$(cat $j | jq '."latencies"."95th" / 1000000')
        P99=$(cat $j | jq '."latencies"."99th" / 1000000')
        MAX=$(cat $j | jq '."latencies"."max" / 1000000')

        DUR=$(cat $j | jq '."duration" / 1000000 ')
        WAIT=$(cat $j | jq '."wait" / 1000000 ')
        NUMREQ=$(cat $j | jq '."requests"')

        RPS=$(echo "$NUMREQ / (( $WAIT + $DUR ) / 1000 )" | bc -l)

        echo "$concurrent, $MEAN, $P50, $P95, $P99, $MAX, $RPS" >> ${outputdir}/LATENCIES-${i}.csv
    done
    cat $scriptdir/plot/vegeta-lats.plot | perl -p -i -e "s#BASE#${outputdir}/LATENCIES-${i}#g;" > ${outputdir}/LATENCIES-${i}.plot
    gnuplot ${outputdir}/LATENCIES-${i}.plot ||:

    cat $scriptdir/plot/vegeta-rps.plot | perl -p -i -e "s#BASE#${outputdir}#g; s#WHICH#${i}#g;" > ${outputdir}/RPS-${i}.plot
    gnuplot ${outputdir}/RPS-${i}.plot ||:

done

# close FDs to ensure tee finishes
exec 1>&0 2>&1
if [ -n "$logging_tee_pid" ];then
    while ps --pid $logging_tee_pid > /dev/null 2>&1
    do
        sleep 1
    done
fi

