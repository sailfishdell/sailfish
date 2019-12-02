#!/bin/sh

set -e
#set -x

# new default 8080 port for this for speed
port=${port:-8080}
continuous=${continuous:-0}
update_seq=${update_seq:-1}
randomize_metrics=${randomize_metrics:-1}
update_timestamps=${update_timestamps:-1}
random_dup_freq=1
random_drop_freq=5
random_switch_freq=20
max_ooo_messages=8
i=${force_seq:--1}

scriptdir=$(cd $(dirname $0); pwd)
. ${scriptdir}/common-vars.sh

if [ -z "$1" ]; then
    echo "need to specify event file"
    exit 1
fi

switcheroo=0
NEXT=0
send_file() {
  file=$1
  file_lines=$(wc -l $file |  awk '{print $1}')
  events_replayed=0
  while read -u 5 line ; do

      # skip comments
      if [[ $line == //* ]]; then
          echo COMMENT, SKIPPING: $line
          continue
      fi

      JQCMD=". "
      JQARGS=""

      # by default, we resequence the file from -1.
      if [[ "$update_seq" -eq 1 ]]; then
        JQCMD+="| .event_seq=\$i"
        JQARGS+="--argjson i $i "

        # increment sequence, but randomly send up to 5 messages in reverse order
        if [[ "$switcheroo" -eq 0 && $NEXT -eq 0 ]]; then
          # randomly 'drop' messages by incrementing the counter by 1 (only when we arent in middle of a switcheroo)
          if [[ "$i" -gt 0 && $RANDOM -lt $(( 32768 * random_drop_freq / 100 )) ]]; then
            echo "   DROP"
            i=$(( i + 1 ))
          fi
          if [[ "$i" -gt 0 && $RANDOM -lt $(( 32768 * random_switch_freq / 100 )) ]]; then
            # send up to 5 messages in reverse order
            switcheroo=$(( $RANDOM / ( 32768 / $max_ooo_messages ) + 1 ))
            echo "   SWITCHEROO: $switcheroo"
            i=$(( i + switcheroo + 1 ))
            NEXT=$(( i + 1 ))
          else
            i=$(( i + 1 ))
          fi
        elif [[ $switcheroo -gt 0 ]]; then
          switcheroo=$(( switcheroo - 1 ))
          i=$(( i - 1 ))
        else
          i=$NEXT
          NEXT=0
        fi
      fi

      if [[ -n "$force_seq" ]]; then
        JQCMD+="| .event_seq=$force_seq"
      fi

      if [[ "$randomize_metrics" -eq 1 ]]; then
        JQCMD+="| if .name == \"MetricValueEvent\" then if .data.Value then .data.Value=\"$RANDOM\" else . end else . end "
      fi

      if [[ "$update_timestamps" -eq 1 ]]; then
        JQCMD+='| if .data.Timestamp then .data.Timestamp=$WWW else . end '
        JQARGS+=" --argjson WWW "\"$(date -u +%Y-%m-%dT%H:%M:%S.%NZ)\"""
      fi

      echo $line | jq $JQARGS "$JQCMD" | $CURLCMD --fail -f $BASE/api/Event%3AInject -d @-

      events_replayed=$(( events_replayed + 1 ))
      total_events_replayed=$(( total_events_replayed + 1 ))
      elapsed=$(( $(date +%s) - start ))
      if [[ $elapsed -ne 0 ]]; then
          EPS=$(( total_events_replayed / elapsed ))
      fi
      echo -n "CURRENT FILE($file): $events_replayed/$file_lines TOTAL: $total_events_replayed/$total_lines   Events per Second: $EPS    SEQ($i)"
      if [[ $switcheroo -gt 0 ]]; then
        echo " SWITCHEROO($switcheroo)"
      else
        echo
      fi

      if [[ -n "$singlestep" ]]; then  read -p "Paused" pause; fi

  done 5<$file
}

start=$(date +%s)
total_events_replayed=0
total_lines=$(wc -l $@ | tail -n1 |  awk '{print $1}' )
EPS="N/A"


while true
do
  for file in "$@"
  do
    send_file $file
  done
  if [ "$continuous" -ne 1 ]; then
    break
  fi
done
