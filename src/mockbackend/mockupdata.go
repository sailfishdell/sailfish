package mockbackend

var initialMockupData = []byte(`{
    "root_links": [
        {"name": "Systems", "target": "/redfish/v1/Systems" },
        {"name": "Chassis", "target": "/redfish/v1/Chassis" },
        {"name": "Tasks", "target": "/redfish/v1/TaskService" },
        {"name": "SessionService", "target": "/redfish/v1/SessionService" },
        {"name": "AccountService", "target": "/redfish/v1/AccountService" },
        {"name": "EventService", "target": "/redfish/v1/EventService" }
    ],
    "root_UUID": "92384634-2938-2342-8820-489239905423",
    "manager_UUID": "58893887-8974-2487-2389-841168418919",
    "redfish_std_copyright": "@Redfish.Copyright: Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.",
    "systemList": [ "437XR1138R2", "dummy"],

    "systems": {
        "437XR1138R2": {
            "name": "WebFrontEnd483",
            "SystemType": "Physical",
            "AssetTag": "Chicago-45Z-2381",
            "Manufacturer": "Contoso",
            "Model": "3500RX",
            "SKU": "8675309",
            "SerialNumber": "437XR1138R2",
            "PartNumber": "224071-J23",
            "Description": "Web Front End node",
            "UUID": "38947555-7742-3448-3784-823347823834",
            "HostName": "web483"
        },
        "dummy": {
            "name": "a dummy system"
        }
    }
}`)

