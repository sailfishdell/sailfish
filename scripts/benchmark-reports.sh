#!/bin/bash

#set -e
set -x

for i in bench/*-walk ; do
    [ -d $i ] || continue
    grep ^Total ${i}/script-output.txt  | sort | grep PIPELINE > ${i}/WALK-TIMING-pipelined.txt
    grep ^Total ${i}/script-output.txt  | sort | grep -v PIPELINE > ${i}/WALK-TIMING-individual.txt
done

for i in bench/*-ab  ; do
    [ -d $i ] || continue
    grep ^Total: ${i}/results-c*-r1000-token.txt  > ${i}/LATENCIES-token.txt
    grep ^Total: ${i}/results-c*-r1000-basic.txt  > ${i}/LATENCIES-basic.txt
    grep ^Request ${i}/results-c*-r1000-token.txt  > ${i}/RPS-token.txt
    grep ^Request ${i}/results-c*-r1000-basic.txt  > ${i}/RPS-basic.txt
done

for i in bench/*-vegeta ; do
    [ -d $i ] || continue
    grep ^Latencies ${i}/basic/report-r*-text.txt > ${i}/LATENCIES-basic.txt
    grep ^Latencies ${i}/token/report-r*-text.txt > ${i}/LATENCIES-token.txt
    grep ^Success ${i}/basic/report-r*-text.txt > ${i}/SUCCESSRATE-basic.txt
    grep ^Success ${i}/token/report-r*-text.txt > ${i}/SUCCESSRATE-token.txt
done
