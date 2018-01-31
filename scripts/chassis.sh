#!/bin/sh

set -x
set -e
unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy

CURLCMD="curl --cacert ./ca.crt"
prot=${prot:-https}
user=${user:-Administrator}
pass=${pass:-password}
host=${host:-localhost}
port=${port:-8443}

URL=$prot://$user:$pass@$host:$port

$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "49467bb4-5c1f-473b-af00-000000000005",
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


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "49467bb4-5c1f-473b-af00-000000000006",
        "Type": "#Power.v1_1_0.Power",
        "Context": "/redfish/v1/$metadata#Power.Power",
        "ResourceURI": "/redfish/v1/Chassis/A33/Power",
        "Privileges": { "GET": ["Login"] },
        "Properties": {
            "Id": "Power",
            "Name": "Power",
            "PowerControl": [{
                "@odata.id": "/redfish/v1/Chassis/A33/Power#/PowerControl/0",
                "MemberId": "0",
                "Name": "System Power Control",
                "PowerConsumedWatts": 224,
                "PowerCapacityWatts": 600,
                "PowerLimit": {
                    "LimitInWatts": 450,
                    "LimitException": "LogEventOnly",
                    "CorrectionInMs": 1000
                },
                "Status": {
                    "State": "Enabled",
                    "Health": "OK"
                }
            }]}
    }'

echo "/redfish/v1/Chassis/A33/Power"
$CURLCMD $URL/redfish/v1/Chassis/A33/Power

$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "49467bb4-5c1f-473b-af00-000000000007",
        "Type": "#Thermal.v1_1_0.Thermal",
        "Context": "/redfish/v1/$metadata#Thermal.Thermal",
        "ResourceURI": "/redfish/v1/Chassis/A33/Thermal",
        "Privileges": { "GET": ["Login"] },
        "Properties": {
            "Id": "Thermal",
            "Name": "Thermal",
            "Temperatures": [
                {
                    "@odata.id": "/redfish/v1/Chassis/A33/Thermal#/Temperatures/0",
                    "MemberId": "0",
                    "Name": "Inlet Temp",
                    "SensorNumber": 42,
                    "Status": {
                        "State": "Enabled",
                        "Health": "OK"
                    },
                    "ReadingCelsius": 25,
                    "UpperThresholdNonCritical": 35,
                    "UpperThresholdCritical": 40,
                    "UpperThresholdFatal": 50,
                    "MinReadingRange": 0,
                    "MaxReadingRange": 200,
                    "PhysicalContext": "Intake"
                },
                {
                    "@odata.id": "/redfish/v1/Chassis/A33/Thermal#/Temperatures/1",
                    "MemberId": "1",
                    "Name": "Board Temp",
                    "SensorNumber": 43,
                    "Status": {
                        "State": "Enabled",
                        "Health": "OK"
                    },
                    "ReadingCelsius": 35,
                    "UpperThresholdNonCritical": 30,
                    "UpperThresholdCritical": 40,
                    "UpperThresholdFatal": 50,
                    "MinReadingRange": 0,
                    "MaxReadingRange": 200,
                    "PhysicalContext": "SystemBoard"
                },
                {
                    "@odata.id": "/redfish/v1/Chassis/A33/Thermal#/Temperatures/2",
                    "MemberId": "2",
                    "Name": "CPU1 Temp",
                    "SensorNumber": 44,
                    "Status": {
                        "State": "Enabled",
                        "Health": "OK"
                    },
                    "ReadingCelsius": 45,
                    "UpperThresholdNonCritical": 60,
                    "UpperThresholdCritical": 82,
                    "MinReadingRange": 0,
                    "MaxReadingRange": 200,
                    "PhysicalContext": "CPU"
                },
                {
                    "@odata.id": "/redfish/v1/Chassis/A33/Thermal#/Temperatures/3",
                    "MemberId": "3",
                    "Name": "CPU2 Temp",
                    "SensorNumber": 45,
                    "Status": {
                        "State": "Enabled",
                        "Health": "OK"
                    },
                    "ReadingCelsius": 46,
                    "UpperThresholdNonCritical": 60,
                    "UpperThresholdCritical": 82,
                    "MinReadingRange": 0,
                    "MaxReadingRange": 200,
                    "PhysicalContext": "CPU"
                }
            ],
            "Fans": [
                {
                    "@odata.id": "/redfish/v1/Chassis/A33/Thermal#/Fans/0",
                    "MemberId": "0",
                    "Name": "BaseBoard System Fan 1",
                    "PhysicalContext": "Backplane",
                    "Status": {
                        "State": "Enabled",
                        "Health": "OK"
                    },
                    "Reading": 2100,
                    "ReadingUnits": "RPM",
                    "UpperThresholdNonCritical": 42,
                    "UpperThresholdCritical": 4200,
                    "UpperThresholdFatal": 42,
                    "LowerThresholdNonCritical": 42,
                    "LowerThresholdCritical": 5,
                    "LowerThresholdFatal": 42,
                    "MinReadingRange": 0,
                    "MaxReadingRange": 5000,
                    "Redundancy": [
                        {
                            "@odata.id": "/redfish/v1/Chassis/A33/Thermal#/Redundancy/0"
                        }
                    ]
                },
                {
                    "@odata.id": "/redfish/v1/Chassis/A33/Thermal#/Fans/1",
                    "MemberId": "1",
                    "Name": "BaseBoard System Fan 2",
                    "PhysicalContext": "Backplane",
                    "Status": {
                        "State": "Enabled",
                        "Health": "OK"
                    },
                    "Reading": 2100,
                    "ReadingUnits": "RPM",
                    "UpperThresholdNonCritical": 42,
                    "UpperThresholdCritical": 4200,
                    "UpperThresholdFatal": 42,
                    "LowerThresholdNonCritical": 42,
                    "LowerThresholdCritical": 5,
                    "LowerThresholdFatal": 42,
                    "MinReadingRange": 0,
                    "MaxReadingRange": 5000,
                    "Redundancy": [
                        {
                            "@odata.id": "/redfish/v1/Chassis/A33/Thermal#/Redundancy/0"
                        }
                    ]
                }
            ],
            "Redundancy": [
                {
                    "@odata.id": "/redfish/v1/Chassis/A33/Thermal#/Redundancy/0",
                    "MemberId": "0",
                    "Name": "BaseBoard System Fans",
                    "RedundancySet": [
                        {
                            "@odata.id": "/redfish/v1/Chassis/A33/Thermal#/Fans/0"
                        },
                        {
                            "@odata.id": "/redfish/v1/Chassis/A33/Thermal#/Fans/1"
                        }
                    ],
                    "Mode": "N+m",
                    "Status": {
                        "State": "Enabled",
                        "Health": "OK"
                    },
                    "MinNumNeeded": 1,
                    "MaxNumSupported": 2
                }
            ]
        }
    }'

echo "/redfish/v1/Chassis/A33/Thermal"
$CURLCMD $URL/redfish/v1/Chassis/A33/Thermal
