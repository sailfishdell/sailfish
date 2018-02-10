#!/bin/sh
set -e
set -x

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

binaries="ocp-server.arm"
[ -n "${binaries}" ] || exit 1
rm -f ${binaries}
time go build -o ocp-server.arm "$@" github.com/superchalupa/go-redfish/cmd/ocp-server

mybox=10.210.137.79
prashanth=10.35.175.208
tester=${prashanth}

scp ${binaries} root@${mybox}:/tmp/
scp ${binaries} root@${prashanth}:/tmp/
ssh root@${tester} /tmp/server.arm ||:
scp root@${tester}:~/ca.crt ./bmc-ca.crt

# prashanth's BMC:
# eth0      Link encap:Ethernet  HWaddr F4:8E:38:CF:13:56  
#           inet addr:10.35.175.208  Bcast:10.35.175.255  Mask:255.255.254.0
