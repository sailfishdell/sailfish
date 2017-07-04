
thermald --emits--> TempChange (data0) --> odata processor --emits--> OdataPropertyUpdated(data) --> redfish server --> (1) update internal model  (2) send redfish events

--> tempchange needs to be introspectable

# address the following
    - methods per odata resource
    - privileges per odata resource  (example: password property)
        - user with configureuser can update username
        - password can only be updated with configureself or system privilege
    - need to be able to extract metadata from entire tree
    - actions:
        - able to specify action endpoints and the allowable parameters so we can formulate a command. This probably ends up being a separate set of "ActionCommand" command/response pairs, but need to be able to automatically generate the (validated?) action command from the core infrastructure.


SAGA:
    - TODO: create odata resource must have required fields: id, context, type
    - when odataresourcecreated event happens, privileges saga goes and attaches privs. probably should look those up in a doc somewhere
    - read looks at privs

METADATA
    - should be able to dynamically create metadata uri from the events as we register odata resources

OUTPUT:
    map [type] manglingfunction
    When Get finds the resource, run it through the optional mangling function, if it exists
    Probably should have the generic output function be aware of per-property privileges

QUESTION:
    how to figure out the CONFIGURE SELF problem?
    I think that for this, just match username against the name property (if either doesn't match, no property)

BASIC AUTH
    - look up against the accounts in account service
    - look up role and then add privs

LOGIN session
    - logging in emits a create odata resource command
    - logging out emits a remove odata resource command
    - login creates a JWT token with privilege list
    - check the time against session timeout
    - session timeout emits the remove odata resource command

PUT/PATCH/POST
    generic handler should check if-match or if-none-match headers
    concept: two types of handlers
        - How do we set up the handlers? HandleMethod property?
        - Generic: 
            operate directly on the aggregates. 
            Emit commands to update properties, 
            know how to read schema to see which fields should be updatable
            side effects outside the odata tree can hang off of the events
        - specific
            marshal json into a "Command"
            set up task object
            send command
            wait for response
            update task
            timeout
            Probably want to ensure we have return events to update properties from external before we complete task


ETAGS
    - etags updated in the property update event



Aggregates
    Odata Resource

Commands:
    CreateOdataResource  -  odataURI
    AddOdataResourceProperty
        - "@odata.id"
        - Members:
    UpdateOdataResourceProperty
    RemoveOdataResourceProperty
    CreateCollection
    AddCollectionMember
    RemoveCollectionMember

Events
    OdataCreated
    OdataPropertyAdded
    OdataPropertyUpdated
    OdataPropertyRemoved

Exceptions:
    OdataAlreadyExists
    PropertyAlreadyExists
    CollectionAlreadyExists
    OdataDoesntExist
    PropertyDoesntExist
    CollectionDoesntExist
