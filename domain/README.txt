
thermald --emits--> TempChange (data0) --> odata processor --emits--> OdataPropertyUpdated(data) --> redfish server --> (1) update internal model  (2) send redfish events

--> tempchange needs to be introspectable

# address the following
    - methods per odataitem
    - privileges per odataitem  (example: password property)
        - user with configureuser can update username
        - password can only be updated with configureself or system privilege
    - need to be able to extract metadata from entire tree
    - actions:
        - able to specify action endpoints and the allowable parameters so we can formulate a command. This probably ends up being a separate set of "ActionCommand" command/response pairs, but need to be able to automatically generate the (validated?) action command from the core infrastructure.

Aggregates
    NewOdata

Commands:
    CreateOdataItem  -  odataURI
    AddOdataItemProperty
        - "@odata.id"
        - Members:
    UpdateOdataItemProperty
    RemoveOdataItemProperty
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
