#!/bin/sh

set -e
set -x

# new default 8080 port for this for speed
port=${port:-8080}

scriptdir=$(cd $(dirname $0); pwd)
. ${scriptdir}/common-vars.sh

if [ -z "$1" ]; then
    echo "need to specify event file"
    exit 1
fi

tmpfile=$(mktemp ./TEMP-XXXXXX)
trap "rm -f $tmpfile" EXIT

events_replayed=0
for file in "$@"
do
  while read -u 5 line ; do
     echo "$line" > $tmpfile

      $CURLCMD --fail -f $BASE/api/Event%3AInject -d  @$tmpfile

      events_replayed=$(( events_replayed + 1 ))
      echo "REPLAYED EVENTS: $events_replayed"

      if [ -n "$singlestep" ]; then  read -p "Paused" pause; fi

  # rate limit requests, if needed:
  #    i=$((i+1))
  #    if [ $((i%20)) -eq 0 ] ; then
  #        sleep 1
  #    fi

  done 5<$file
done
