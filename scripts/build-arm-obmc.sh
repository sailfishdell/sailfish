#!/bin/sh
set -e
set -x

scriptdir=$(cd $(dirname $0); pwd)
cd $scriptdir/../

[ -e test-machine.conf ] && . ./test-machine.conf

build_spacemonkey=${build_spacemonkey:-0}
if [ "$build_spacemonkey" -ne 0 ]; then
    BUILD_TAGS="$BUILD_TAGS spacemonkey"
fi
build_simulation=${build_simulation:-0}
if [ "$build_simulation" -ne 0 ]; then
    BUILD_TAGS="$BUILD_TAGS simulation"
else
    BUILD_TAGS="$BUILD_TAGS openbmc"
fi

YOCTO_SYSROOTS_BASE=${YOCTO_SYSROOTS_BASE:-~/openbmc/build/tmp/sysroots}
PLATFORM=evb-npcm750

CROSS_PATH=${CROSS_PATH:-${YOCTO_SYSROOTS_BASE}/${PLATFORM}}
CROSS_SYSROOT=${CROSS_SYSROOT:-${YOCTO_SYSROOTS_BASE}/x86_64-linux}
export PKG_CONFIG_PATH=${CROSS_PATH}/usr/lib/pkgconfig/

# sort of hardcoded to the yocto paths for this specific version, oh well
export PATH=${CROSS_SYSROOT}/usr/lib/arm-openbmc-linux-gnueabi/go/bin/:${CROSS_SYSROOT}/usr/libexec/arm-openbmc-linux-gnueabi/gcc/arm-openbmc-linux-gnueabi/6.2.0/:${PATH}

export GOARCH=arm
export GOARM=5
export GOOS=linux

binaries=${binaries:-"ocp-server"}
for pkg in $binaries
do
    rm -f ${pkg}.${GOARCH}
    time go build -tags "$BUILD_TAGS" -o ${pkg}.${GOARCH}   "$@" github.com/superchalupa/go-redfish/cmd/${pkg}
done

# build mappercli
pkg=mappercli
rm -f ${pkg}.${GOARCH}
time go build -tags "$BUILD_TAGS" -o ${pkg}.${GOARCH} "$@" github.com/superchalupa/go-redfish/cmd/${pkg}

for box in ${TEST_MACHINES}
do
    for binary in ${binaries}
    do
        scp ${binary}.${GOARCH} root@${box}:/tmp/
    done
    ssh root@${box} /tmp/ocp-server.${GOARCH} ||:
    scp root@${box}:~/ca.crt ./${box}-ca.crt
done
