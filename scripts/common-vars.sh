#!/bin/sh

set -e
set -x

unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy
scriptdir=$(cd $(dirname $0); pwd)

user=${user:-Administrator}
pass=${pass:-password}
host=${host:-localhost}
port=${port:-8443}

if [ "${host}" = "localhost" ]; then
    cacert=${cacert:-./ca.crt}
else
    cacert=${cacert:-./${host}-ca.crt}
fi

host=${host:-localhost}
if [ "${port}" = "443" -o "${port}" = "8443" ]; then
    prot=${prot:-https}
else
    prot=${prot:-http}
fi
BASE=${prot}://${host}:${port}
START_URL=${START_URL:-"/redfish/v1"}

timingarg="\nTotal request time: %{time_total} seconds for url: %{url_effective}\n"
CURLCMD="curl --cacert ${cacert} ${CURL_OPTS} -L "

set_auth_header() {
    if [ -z "$AUTH_HEADER" ]; then
        if [ -n "$TOKEN" ]; then
            AUTH_HEADER="Authorization: Bearer $TOKEN"
        elif [ -n "$X_AUTH_TOKEN" ]; then
            export AUTH_HEADER="X-Auth-Token: $X_AUTH_TOKEN"
        else
            eval $(scripts/login.sh $user $pass)
        fi
    fi
}



