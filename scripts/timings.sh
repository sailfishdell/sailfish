#!/bin/sh    
    
for i in "" / /v1/ /v1/Systems /v1/Systems/437XR1138R2 /v1/Systems/dummy
do
    curl -w "@curl-format.txt" http://localhost:8080/redfish${i}   
done

