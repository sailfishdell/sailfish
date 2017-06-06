#!/bin/sh

curl -L -w"\nTotal request time: %{time_total} seconds\n" "$@"
