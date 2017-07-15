#!/bin/sh

username=$1
password=$2

export X_AUTH_TOKEN=$(curl -v -D ./test http://localhost:8080/redfish/v1/SessionService/Sessions -X POST -d "{\"UserName\": \"${username}\", \"Password\": \"${password}\"}" 2>&1 | grep X-Auth-Token | cut -d: -f2 | perl -p -e 's/\r//g;')

for i in $X_AUTH_TOKEN
do
    X_AUTH_TOKEN=$i
    break
done

if [ -n "$X_AUTH_TOKEN" ]; then
    echo "X_AUTH_TOKEN=$X_AUTH_TOKEN"
    echo "AUTH_HEADER='X-Auth-Token: $X_AUTH_TOKEN'"
else
    echo "X_AUTH_TOKEN="
fi
