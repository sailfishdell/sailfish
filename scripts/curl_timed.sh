#!/bin/sh

curl -s -L -w"\nTotal request time: %{time_total} seconds\n" "$@"
