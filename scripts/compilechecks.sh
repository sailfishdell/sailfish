#!/bin/bash

#set -e
#set -x

scriptdir=$(cd $(dirname $0); pwd)
topdir=$scriptdir/..
cd $topdir

# To install staticcheck:
# go get honnef.co/go/tools/cmd/staticcheck

for dir in $(find ./cmd ./src/http_inject ./src/log ./src/ocp/am3 ./src -depth -type d )
do
  if [[ $dir = ./cmd/metric-engine/cgo ]]; then
    # can't run compile checks here without cross tools
    continue
  fi

  if ! ls $dir/*go > /dev/null 2>&1; then
    # no go files
    echo =============== SKIPPING $DIR: No GO files
    continue
  fi

  echo =============== GO VET $dir
  go vet -mod=vendor $dir
  echo =============== STATICCHECK $dir
  staticcheck $dir
done

exit 0
echo =============== GO LINT cmd/...
#golint cmd/...
echo =============== GO LINT src/...
#golint src/...
