#!/bin/sh

set -e
#set -x

# new default 8080 port for this for speed
port=${port:-8080}
forceseq=${forceseq:-1}

scriptdir=$(cd $(dirname $0); pwd)
. ${scriptdir}/common-vars.sh

if [ -z "$1" ]; then
    echo "need to specify event file"
    exit 1
fi

tmpfile=$(mktemp ./TEMP-XXXXXX)
trap "rm -f $tmpfile TMP" EXIT
i=-1
start=$(date +%s)
total_events_replayed=0
total_lines=$(wc -l $@ | tail -n1 |  awk '{print $1}' )
EPS="N/A"
for file in "$@"
do
  file_lines=$(wc -l $file |  awk '{print $1}')
  events_replayed=0
  while read -u 5 line ; do
      if [ "$forceseq" -ne "1" ]; then
        i=$( echo $line | jq '.event_seq' )
      fi
      NOW=$(date --iso-8601=ns)
      echo $line |
        jq  --argjson i "$i" ".event_seq=\$i" |
        jq  --argjson WWW "\"$(date -u +%Y-%m-%dT%H:%M:%S.%NZ)\"" '. | if .data.Timestamp then .data.Timestamp=$WWW else . end' |
        jq  ". | if .data.MetricValue then .data.MetricValue=\"$RANDOM\" else . end" |
        $CURLCMD --fail -f $BASE/api/Event%3AInject -d @-

      i=$(($i+1))


      events_replayed=$(( events_replayed + 1 ))
      total_events_replayed=$(( total_events_replayed + 1 ))
      elapsed=$(( $(date +%s) - start ))
      if [ $elapsed -ne 0 ]; then
          EPS=$(( total_events_replayed / elapsed ))
      fi
      echo "CURRENT FILE($file): $events_replayed/$file_lines   TOTAL: $total_events_replayed/$total_lines   Events per Second: $EPS"

      if [ -n "$singlestep" ]; then  read -p "Paused" pause; fi

  done 5<$file
done
