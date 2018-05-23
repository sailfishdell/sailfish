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
        "ID": "371d0e7d-21b9-4f74-bc22-000000000000",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/SlotConfigs",
        "Type": "#DellSlotConfigsCollection.DellSlotConfigsCollection",
        "Context": "/redfish/v1/$metadata#DellSlotConfigsCollection.DellSlotConfigsCollection",
        "Privileges": { "GET": ["Unauthenticated"] },
        "Collection": true,
        "Properties": {
            "Name": "DellSlotConfigsCollection"
        }
    }'

$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000001",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/SlotConfigs/SlotConfig.1",
        "Type": "#DellSlotConfig.v1_0_0.DellSlotConfig",
        "Context": "/redfish/v1/$metadata#DellSlotConfig.DellSlotConfig",
        "Privileges": { "GET": ["Unauthenticated"], "PATCH":["ConfigureManager"] },
        "Properties": {
             "Rows": "1",
             "Name": "SlotConfig.1",
             "Order": "LR",
             "Location": "rear",
             "ParentConfig": "",
             "Type": "CMC",
             "Id": "SlotConfig.1",
             "Columns": "2",
             "Orientation": "Horizontal"
        }
    }'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000002",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/SlotConfigs/SlotConfig.2",
        "Type": "#DellSlotConfig.v1_0_0.DellSlotConfig",
        "Context": "/redfish/v1/$metadata#DellSlotConfig.DellSlotConfig",
        "Privileges": { "GET": ["Unauthenticated"], "PATCH":["ConfigureManager"] },
        "Properties": {
             "Rows": "1",
             "Name": "SlotConfig.2",
             "Order": "LR",
             "Location": "front",
             "ParentConfig": "",
             "Type": "Sled",
             "Id": "SlotConfig.2",
             "Columns": "4",
             "Orientation": "Vertical"
        }
    }'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000003",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/SlotConfigs/SlotConfig.3",
        "Type": "#DellSlotConfig.v1_0_0.DellSlotConfig",
        "Context": "/redfish/v1/$metadata#DellSlotConfig.DellSlotConfig",
        "Privileges": { "GET": ["Unauthenticated"], "PATCH":["ConfigureManager"] },
        "Properties": {
             "Rows": "1",
             "Name": "SlotConfig.3",
             "Order": "LR",
             "Location": "front",
             "ParentConfig": "",
             "Type": "Sled",
             "Id": "SlotConfig.3",
             "Columns": "4",
             "Orientation": "Vertical"
        }
    }'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000004",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/SlotConfigs/SlotConfig.4",
        "Type": "#DellSlotConfig.v1_0_0.DellSlotConfig",
        "Context": "/redfish/v1/$metadata#DellSlotConfig.DellSlotConfig",
        "Privileges": { "GET": ["Unauthenticated"], "PATCH":["ConfigureManager"] },
        "Properties": {
             "Rows": "4",
             "Name": "SlotConfig.4",
             "Order": "TB",
             "Location": "rear",
             "ParentConfig": "",
             "Type": "Fan",
             "Id": "SlotConfig.4",
             "Columns": "1",
             "Orientation": "Vertical"
        }
    }'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000005",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/SlotConfigs/SlotConfig.5",
        "Type": "#DellSlotConfig.v1_0_0.DellSlotConfig",
        "Context": "/redfish/v1/$metadata#DellSlotConfig.DellSlotConfig",
        "Privileges": { "GET": ["Unauthenticated"], "PATCH":["ConfigureManager"] },
        "Properties": {
             "Rows": "1",
             "Name": "SlotConfig.5",
             "Order": "LR",
             "Location": "rear",
             "ParentConfig": "",
             "Type": "Fan",
             "Id": "SlotConfig.5",
             "Columns": "5",
             "Orientation": "Horizontal"
        }
    }'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000006",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/SlotConfigs/SlotConfig.6",
        "Type": "#DellSlotConfig.v1_0_0.DellSlotConfig",
        "Context": "/redfish/v1/$metadata#DellSlotConfig.DellSlotConfig",
        "Privileges": { "GET": ["Unauthenticated"], "PATCH":["ConfigureManager"] },
        "Properties": {
             "Rows": "1",
             "Name": "SlotConfig.6",
             "Order": "LR",
             "Location": "front",
             "ParentConfig": "",
             "Type": "PSU",
             "Id": "SlotConfig.6",
             "Columns": "6",
             "Orientation": "Horizontal"
        }
    }'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000007",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/SlotConfigs/SlotConfig.7",
        "Type": "#DellSlotConfig.v1_0_0.DellSlotConfig",
        "Context": "/redfish/v1/$metadata#DellSlotConfig.DellSlotConfig",
        "Privileges": { "GET": ["Unauthenticated"], "PATCH":["ConfigureManager"] },
        "Properties": {
             "Rows": "2",
             "Name": "SlotConfig.7",
             "Order": "TB",
             "Location": "rear",
             "ParentConfig": "",
             "Type": "IOM",
             "Id": "SlotConfig.7",
             "Columns": "1",
             "Orientation": "Horizontal"
        }
    }'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000008",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/SlotConfigs/SlotConfig.8",
        "Type": "#DellSlotConfig.v1_0_0.DellSlotConfig",
        "Context": "/redfish/v1/$metadata#DellSlotConfig.DellSlotConfig",
        "Privileges": { "GET": ["Unauthenticated"], "PATCH":["ConfigureManager"] },
        "Properties": {
             "Rows": "2",
             "Name": "SlotConfig.8",
             "Order": "TB",
             "Location": "rear",
             "ParentConfig": "",
             "Type": "IOM",
             "Id": "SlotConfig.8",
             "Columns": "1",
             "Orientation": "Horizontal"
        }
    }'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000009",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/SlotConfigs/SlotConfig.9",
        "Type": "#DellSlotConfig.v1_0_0.DellSlotConfig",
        "Context": "/redfish/v1/$metadata#DellSlotConfig.DellSlotConfig",
        "Privileges": { "GET": ["Unauthenticated"], "PATCH":["ConfigureManager"] },
        "Properties": {
             "Rows": "1",
             "Name": "SlotConfig.9",
             "Order": "LR",
             "Location": "rear",
             "ParentConfig": "",
             "Type": "IOM",
             "Id": "SlotConfig.9",
             "Columns": "2",
             "Orientation": "Horizontal"
        }
    }'



$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-100000000010",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots",
        "Type": "#DellSlotsCollection.DellSlotsCollection",
        "Context": "/redfish/v1/$metadata#DellSlotsCollection.DellSlotsCollection",
        "Privileges": { "GET": ["Unauthenticated"] },
        "Collection": true,
        "Properties": {
            "Name": "DellSlotsCollection"
        }
    }'


# the following ones need to be hooked to a model. (ugh)

$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000011",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/CMCSlot.1",
        "Type": "#DellSlot.v1_0_0.DellSlot",
        "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
        "Privileges": { "GET": ["Unauthenticated"] },
        "Properties": {
             "Name": "1",
             "Occupied": "True",
             "Contains": "CMC.Integrated.1",
             "Config": "SlotConfig.1",
             "Id": "CMCSlot.1",
             "SlotName": "MM1"
        }
    }'



$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000012",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/CMCSlot.2",
        "Type": "#DellSlot.v1_0_0.DellSlot",
        "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
        "Privileges": { "GET": ["Unauthenticated"] },
        "Properties": {
             "Name": "2",
             "Occupied": "True",
             "Contains": "CMC.Integrated.2",
             "Config": "SlotConfig.1",
             "Id": "CMCSlot.2",
             "SlotName": "MM2"
        }
    }'



$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000013",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/FanSlot.1",
        "Type": "#DellSlot.v1_0_0.DellSlot",
        "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
        "Privileges": { "GET": ["Unauthenticated"] },
        "Properties": {
             "Name": "FAN-1",
             "Occupied": "True",
             "Contains": "Fan.Slot.1",
             "Config": "SlotConfig.4",
             "Id": "FanSlot.1",
             "SlotName": "1"
        }
    }'



$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000014",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/FanSlot.2",
        "Type": "#DellSlot.v1_0_0.DellSlot",
        "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
        "Privileges": { "GET": ["Unauthenticated"] },
        "Properties": {
             "Name": "FAN-2",
             "Occupied": "True",
             "Contains": "Fan.Slot.2",
             "Config": "SlotConfig.4",
             "Id": "FanSlot.2",
             "SlotName": "2"
        }
    }'



$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000015",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/FanSlot.3",
        "Type": "#DellSlot.v1_0_0.DellSlot",
        "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
        "Privileges": { "GET": ["Unauthenticated"] },
        "Properties": {
             "Name": "FAN-3",
             "Occupied": "True",
             "Contains": "Fan.Slot.3",
             "Config": "SlotConfig.4",
             "Id": "FanSlot.3",
             "SlotName": "3"
        }
    }'



$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000016",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/FanSlot.4",
        "Type": "#DellSlot.v1_0_0.DellSlot",
        "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
        "Privileges": { "GET": ["Unauthenticated"] },
        "Properties": {
             "Name": "FAN-4",
             "Occupied": "True",
             "Contains": "Fan.Slot.4",
             "Config": "SlotConfig.4",
             "Id": "FanSlot.4",
             "SlotName": "4"
        }
    }'



$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000017",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/FanSlot.5",
        "Type": "#DellSlot.v1_0_0.DellSlot",
        "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
        "Privileges": { "GET": ["Unauthenticated"] },
        "Properties": 
        {
             "Name": "FAN-5",
             "Occupied": "True",
             "Contains": "Fan.Slot.5",
             "Config": "SlotConfig.5",
             "Id": "FanSlot.5",
             "SlotName": "5"
        }
    }'



$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000018",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/FanSlot.6",
        "Type": "#DellSlot.v1_0_0.DellSlot",
        "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
        "Privileges": { "GET": ["Unauthenticated"] },
        "Properties": 
        {
             "Name": "FAN-6",
             "Occupied": "True",
             "Contains": "Fan.Slot.6",
             "Config": "SlotConfig.5",
             "Id": "FanSlot.6",
             "SlotName": "6"
        }
    }'



$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000019",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/FanSlot.7",
        "Type": "#DellSlot.v1_0_0.DellSlot",
        "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
        "Privileges": { "GET": ["Unauthenticated"] },
        "Properties": 
        {
             "Name": "FAN-7",
             "Occupied": "True",
             "Contains": "Fan.Slot.7",
             "Config": "SlotConfig.5",
             "Id": "FanSlot.7",
             "SlotName": "7"
        }
    }'



$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000020",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/FanSlot.8",
        "Type": "#DellSlot.v1_0_0.DellSlot",
        "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
        "Privileges": { "GET": ["Unauthenticated"] },
        "Properties": 
        {
             "Name": "FAN-8",
             "Occupied": "True",
             "Contains": "Fan.Slot.8",
             "Config": "SlotConfig.5",
             "Id": "FanSlot.8",
             "SlotName": "8"
        }
    }'



$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000021",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/FanSlot.9",
        "Type": "#DellSlot.v1_0_0.DellSlot",
        "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
        "Privileges": { "GET": ["Unauthenticated"] },
        "Properties": 
        {
             "Name": "FAN-9",
             "Occupied": "True",
             "Contains": "Fan.Slot.9",
             "Config": "SlotConfig.5",
             "Id": "FanSlot.9",
             "SlotName": "9"
        }
    }'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "371d0e7d-21b9-4f74-bc22-000000000022",
        "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/IOMSlot.1",
        "Type": "#DellSlot.v1_0_0.DellSlot",
        "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
        "Privileges": { "GET": ["Unauthenticated"] },
        "Properties": 
        {
             "Name": "A1",
             "Occupied": "True",
             "Contains": "IOM.Slot.A1",
             "Config": "SlotConfig.7",
             "Id": "IOMSlot.1",
             "SlotName": "IOM-A1"
        }
    }'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000023",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/IOMSlot.2",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "A2",
         "Occupied": "True",
         "Contains": "IOM.Slot.A2",
         "Config": "SlotConfig.7",
         "Id": "IOMSlot.2",
         "SlotName": "IOM-A2"
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000024",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/IOMSlot.3",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "B1",
         "Occupied": "True",
         "Contains": "IOM.Slot.B1",
         "Config": "SlotConfig.8",
         "Id": "IOMSlot.3",
         "SlotName": "IOM-B1"
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000025",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/IOMSlot.4",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "B2",
         "Occupied": "True",
         "Contains": "IOM.Slot.B2",
         "Config": "SlotConfig.8",
         "Id": "IOMSlot.4",
         "SlotName": "IOM-B2"
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000026",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/IOMSlot.5",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "C1",
         "Occupied": "True",
         "Contains": "IOM.Slot.C1",
         "Config": "SlotConfig.9",
         "Id": "IOMSlot.5",
         "SlotName": "IOM-C1"
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000027",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/IOMSlot.6",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "C2",
         "Occupied": "True",
         "Contains": "IOM.Slot.C2",
         "Config": "SlotConfig.9",
         "Id": "IOMSlot.6",
         "SlotName": "IOM-C2"
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000030",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/PSUSlot.1",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "",
         "Occupied": "True",
         "Contains": "PSU.Slot.1",
         "Config": "SlotConfig.6",
         "Id": "PSUSlot.1",
         "SlotName": "1"
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000031",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/PSUSlot.2",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "",
         "Occupied": "True",
         "Contains": "PSU.Slot.2",
         "Config": "SlotConfig.6",
         "Id": "PSUSlot.2",
         "SlotName": "2"
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000032",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/PSUSlot.3",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "",
         "Occupied": "True",
         "Contains": "PSU.Slot.3",
         "Config": "SlotConfig.6",
         "Id": "PSUSlot.3",
         "SlotName": "3"
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000033",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/PSUSlot.4",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "",
         "Occupied": "True",
         "Contains": "PSU.Slot.4",
         "Config": "SlotConfig.6",
         "Id": "PSUSlot.4",
         "SlotName": ""
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000034",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/PSUSlot.5",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "",
         "Occupied": "True",
         "Contains": "PSU.Slot.5",
         "Config": "SlotConfig.6",
         "Id": "PSUSlot.5",
         "SlotName": "5"
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000035",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/PSUSlot.6",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "",
         "Occupied": "True",
         "Contains": "PSU.Slot.6",
         "Config": "SlotConfig.6",
         "Id": "PSUSlot.6",
         "SlotName": "6"
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000036",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/SledSlot.1",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "1",
         "Occupied": "True",
         "Contains": "System.Modular.1",
         "SledProfile": "",
         "Config": "SlotConfig.2",
         "Id": "SledSlot.1",
         "SlotName": "Sled-1pf"
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000037",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/SledSlot.2",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "2",
         "Occupied": "True",
         "Contains": "System.Modular.2",
         "SledProfile": "",
         "Config": "SlotConfig.2",
         "Id": "SledSlot.2",
         "SlotName": "Sled-2pf"
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000040",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/SledSlot.3",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "3",
         "Occupied": "True",
         "Contains": "System.Modular.3",
         "SledProfile": "",
         "Config": "SlotConfig.2",
         "Id": "SledSlot.3",
         "SlotName": "Sled-3pf"
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000041",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/SledSlot.4",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "4",
         "Occupied": "True",
         "Contains": "System.Modular.4",
         "SledProfile": "",
         "Config": "SlotConfig.2",
         "Id": "SledSlot.4",
         "SlotName": "Sled-4pf"
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000042",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/SledSlot.5",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "5",
         "Occupied": "True",
         "Contains": "System.Modular.5",
         "SledProfile": "",
         "Config": "SlotConfig.3",
         "Id": "SledSlot.5",
         "SlotName": "Sled-5pf"
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000043",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/SledSlot.6",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "6",
         "Occupied": "True",
         "Contains": "System.Modular.6",
         "SledProfile": "",
         "Config": "SlotConfig.3",
         "Id": "SledSlot.6",
         "SlotName": "Sled-6pf"
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000044",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/SledSlot.7",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "7",
         "Occupied": "True",
         "Contains": "System.Modular.7",
         "SledProfile": "",
         "Config": "SlotConfig.3",
         "Id": "SledSlot.7",
         "SlotName": "Sled-7s"
    }}'


$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
     "ID": "371d0e7d-21b9-4f74-bc22-000000000045",
     "ResourceURI": "/redfish/v1/Chassis/System.Chassis.1/Slots/SledSlot.8",
     "Type": "#DellSlot.v1_0_0.DellSlot",
     "Context": "/redfish/v1/$metadata#DellSlot.DellSlot",
     "Privileges": { "GET": ["Unauthenticated"] },
     "Properties": {
         "Name": "8",
         "Occupied": "True",
         "Contains": "System.Modular.8",
         "SledProfile": "",
         "Config": "SlotConfig.3",
         "Id": "SledSlot.8",
         "SlotName": "Sled-8s"
    }}'


