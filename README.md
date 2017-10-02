
Proposed design:

1) On startup, read initial redfish tree template. This redfish tree template contains both the actual redfish data that is output as well as metadata that describes how to populate and update that data. Below is a truncated example:

ComputerSystem example:
{
    "@meta": { "supported_methods": ["GET", "PUT", "PATCH", "POST"], },

    "@odata.type": "#ComputerSystem.v1_1_0.ComputerSystem",
    "Id": "",

    // Below is the actual redfish data that is output. Initial values are generally undefined, however, we have the option to hardcode certain values (for development or test), or save cached data here.
    "SerialNumber": "<error retrieving serial number>", // default value

    // Below is the metadata for this data item that control how the redfish core operates on this data.
    "SerialNumber@meta": {
        // Cache control: by default, redfish core caches data. This can be turned off on a per-item basis, or we can specify a max cache age.
        //  - refresh_after: non-blocking background refresh data on GET if cached value is older than refresh_after, but less than max_age_seconds. 
        //  - max_age_seconds: Blocking refresh from back end if age > max_age_seconds.
        // These options serve to increase performance of the redfish stack as well as reduce load on the system. In the face of a redfish stress, we will avoid many calls back into DM/AR/etc and serve from cache in the redfish stack.
        "cache": { "enabled": "True", "max_age_seconds": 0, "refresh_after": 0, "query_timeout": 0,
            // internal only (calculated values):
            "etag": "xxxx",
            "expires_at": "<unix time>"
            },

        // Query is how the redfish core obtains the data the first time that
        // it needs it. For example, it can query data manager or attribute
        // registry. By default, the data is cached forever. The first time that
        // this redfish item is requested, it performs this query and caches the
        // result (per cache control directive, above). If the data has events
        // that are published on changes, then this should be sufficient (see
        // 'eventfilter' below), as the data will be updated when events are
        // recieved.
        "query": { "type": "AR", "query": "#System.Chassis.1#Info.1#ServiceTag" },

        "eventfilter": { "type": "AR", "filter": "#System.Chassis.1#Info.1#ServiceTag", },

        // Get privileges from AR record or internal privilege map
        "privileges": { "type": "AR" }
        "PATCH": { "type": "AR", "update": "#System.Chassis.1#Info.1#ServiceTag" },
        },

    "Name": "<error retrieving name>",   // default value
    "Name@meta": { 
        "cache": { "enabled": "True", "max_age_seconds": 0, "refresh_after": 0, "query_timeout": 0,
        "query": { "type": "AR", "query": "#System.Chassis.1#Info.1#Name" },
        "eventfilter": { "type": "AR", "filter": "#System.Chassis.1#Info.1#Name", },
    },

    "Status": {
        "State": "Enabled",
        "State@meta": { },
        "Health": "OK",
        "Health@meta": {},
        "HealthRollUp": "OK",
        "HealthRollUp@meta": {}
    },
}


ComputerSystemCollection example:

{
    "@odata.type":"#ComputerSystemCollection.ComputerSystemCollection",
    "@odata.context":"/redfish/v1/$metadata#ComputerSystemCollection.ComputerSystemCollection",
    "@odata.id":"/redfish/v1/Systems"
    "Name":"Computer System Collection",
    "Members": [],
    // "Members@odata.count":4, // this would be automatically maintained by the query below
    "Members@meta": {
        "type": "array", //automatically add @odata.count to count array
        "cache": {}
        "query": { "type": "AR", "select": "System.Modular.*#Info .[] | {\"@odata.id\": \"/redfish/v1/Systems/\" .LastServiceTag \"}" }
        "eventfilter": [
            { "type": "AR", "action": "add", },  // event that adds items to array
            { "type": "AR", "action": "remove"}, // event that removes items from the array
            ]
    }
}


2) Runtime
    - On GET, if local value has cached value that is not expired, run "query" to get the result.
    - event filter: process incoming stream of events and update redfish data
    - Caching: all values cached by default, with easy override for things that can't be cached. Normal operation is A) Initial get/query, and then B) update local cache based on events.






1) Read in tree of template redfish content
    - One tree object per redfish tree that we support
        - Could run this as a microservice that mirrors other servers
    - Each redfish URI is one Aggregate
    - Read repository has tree object that maps to read side uri objects
    - Command side needs to have the object id in the command

2) Startup saga:
    - Run queries to populate initial data

3) external events -> SAGA -> Commands
    - 

3) output filter:
    - Run commands on output to update
        - read side gets obj id
        - sends event with request to ouptut saga
        - output saga sends output event
