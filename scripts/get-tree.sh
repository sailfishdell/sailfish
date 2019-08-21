#!/bin/bash

set -x
set -e

TREE=${TREE:-$HOME}

unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy

TMP=$(mktemp /dev/shm/curl_input.XXXXXXXXX)

CURLCMD="curl "
prot=${prot:-http}
host=${host:-localhost}
port=${port:-443}

URL=$prot://$host:$port

add_element()
{
	directory=$(dirname $1)
	URI=${directory#${TREE}}
	URI=${URI%/}
	UUID=$(uuidgen)
	ELEMENT="$(cat $1)"
	TYPE=$(echo $ELEMENT | jq '.["@odata.type"]')
	CONTEXT=$(echo $ELEMENT | jq '.["@odata.context"]')

	size=$(echo $ELEMENT | wc -c)
	echo SIZE=$size

     echo "{"  > $TMP
     echo \"ID\": \"$UUID\", >> $TMP
     echo \"ResourceURI\": \"${URI}\", >> $TMP
     echo \"Type\": $TYPE, >> $TMP
     echo \"Context\": $CONTEXT,>> $TMP
     echo \"Privileges\": { \"GET\": [\"Unauthenticated\"], \"PATCH\": [\"ConfigureManager\"]}, >> $TMP

     echo \"Properties\": >> $TMP
     cat $1 >> $TMP
     echo "}" >> $TMP
			
	$CURLCMD $URL/api/RedfishResource%3ACreate  -d @$TMP
    echo  RET=$?
}

for signal in 1 2 3 5 6 15;do
        trap "rm -f $TMP" $signal
done


for i in $(find \
	${TREE}/redfish/v1 -type f -name index.json \
	|grep -E -v 'AccountService|SessionService|EventService' | grep -v 'redfish/v1/Managers/index.json | grep -v redfish/v1/Systems/index.json |  grep -v redfish/v1/Chassis/index.json' )
do
	echo $i
	add_element $i
done
 
rm -f $TMP
