#!/bin/sh
set -e
set -x

mybox=10.210.137.79
prashanth=10.35.175.208
DRB_List=${prashanth}

YOCTO_SYSROOTS_BASE=${YOCTO_SYSROOTS_BASE:-~/openbmc/repo/build/tmp/sysroots}
PLATFORM=evb-npcm750

CROSS_PATH=${CROSS_PATH:-${YOCTO_SYSROOTS_BASE}/${PLATFORM}}
CROSS_SYSROOT=${CROSS_SYSROOT:-${YOCTO_SYSROOTS_BASE}/x86_64-linux}
export PKG_CONFIG_PATH=${CROSS_PATH}/usr/lib/pkgconfig/

# sort of hardcoded to the yocto paths for this specific version, oh well
export PATH=${CROSS_SYSROOT}/usr/lib/arm-openbmc-linux-gnueabi/go/bin/:${CROSS_SYSROOT}/usr/libexec/arm-openbmc-linux-gnueabi/gcc/arm-openbmc-linux-gnueabi/6.2.0/:${PATH}

export GOARCH=arm
export GOARM=5
export GOOS=linux

binaries=${binaries:-"ocp-server mappercli"}
for pkg in $binaries
do
    rm -f ${pkg}.${GOARCH}
    time go build -o ${pkg}.${GOARCH}   "$@" github.com/superchalupa/go-redfish/cmd/${pkg}
done

for box in ${DRB_List}
do
    for binary in ${binaries}
    do
        scp ${binary}.${GOARCH} root@${box}:/tmp/
    done
    if ssh root@${box} /tmp/ocp-server.${GOARCH}; then
        scp root@${box}:~/ca.crt ./${box}-ca.crt
    fi
done
