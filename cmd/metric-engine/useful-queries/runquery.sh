#!/bin/sh

scriptdir=$(cd $(dirname $0); pwd)

DB=${DB:-/run/telemetryservice/telemetry_timeseries_database.db}
if [ -e ${1} ]; then
  # user specified db path
  DB=${1}
  shift
fi

for query in "$@"
do
  for d in ${scriptdir}/ /usr/share/metric-engine/ /tmp/
  do
    if [ -e ${d}/${query}.sql ]; then
      echo RUNNING QUERY: $query
      sqlite3 $DB < ${d}/${query}.sql
      break
    fi
  done
done
