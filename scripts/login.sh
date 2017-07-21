#!/bin/sh

username=$1
password=$2

export X_AUTH_TOKEN=$(curl -v http://localhost:8080/redfish/v1/SessionService/Sessions -X POST -d "{\"UserName\": \"${username}\", \"Password\": \"${password}\"}" 2>&1 | grep X-Auth-Token | cut -d: -f2 | perl -p -e 's/\r//g;')

for i in $X_AUTH_TOKEN
do
    X_AUTH_TOKEN=$i
    break
done

if [ -n "$X_AUTH_TOKEN" ]; then
    echo "export X_AUTH_TOKEN=$X_AUTH_TOKEN"
    echo "export AUTH_HEADER='X-Auth-Token: $X_AUTH_TOKEN'"
else
    echo "export X_AUTH_TOKEN="
    echo "export AUTH_HEADER="
fi
