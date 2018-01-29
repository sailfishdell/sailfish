#!/bin/sh

CURLCMD="curl --cacert ./ca.crt"
prot=https
user=Administrator
pass=password
host=localhost
port=8443

URL=$prot://$user:$pass@$host:$port

$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "49467bb4-5c1f-473b-af00-5d4fa209f7bf",
        "Type": "#Chassis.v1_2_0.Chassis",
        "Context": "/redfish/v1/$metadata#Chassis.Chassis",
        "Privileges": { "GET": ["Login"] },

        "ResourceURI":"/redfish/v1/Chassis/A33",
        "Properties": {
            "Name": "Catfish System Chassis",
            "Id": "A33",
            "ChassisType": "RackMount",
            "Manufacturer": "CatfishManufacturer",
            "Model": "YellowCat1000",

            "SerialNumber": "2M220100SL",
            "SerialNumber@meta": {"GET": {"plugin": ""}},

            "SKU": "",
            "PartNumber": "",
            "AssetTag": "CATFISHASSETTAG",
            "IndicatorLED": "Lit",
            "PowerState": "On",
            "Status": {
                "State": "Enabled",
                "Health": "OK"
            },

            "Thermal": { "@odata.id": "/redfish/v1/Chassis/A33/Thermal" },
            "Power": { "@odata.id": "/redfish/v1/Chassis/A33/Power" },
            "Links": {
                "ComputerSystems": [ { "@odata.id": "/redfish/v1/Systems/2M220100SL" } ],
                "ManagedBy": [ { "@odata.id": "/redfish/v1/Managers/bmc" } ],
                "ManagersInChassis": [ { "@odata.id": "/redfish/v1/Managers/bmc" } ]
            }
        }
    }'


echo "/redfish/v1/Chassis/A33"
$CURLCMD $URL/redfish/v1/Chassis/A33

