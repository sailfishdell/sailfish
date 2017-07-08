
(from http://cqrs.nu/Faq/commands-and-events)
What is an event?

An event represents something that took place in the domain. They are always named with a past-participle verb, such as OrderConfirmed. It's not unusual, but not required, for an event to name an aggregate or entity that it relates to; let the domain language be your guide.

Since an event represents something in the past, it can be considered a statement of fact and used to take decisions in other parts of the system.

What is a command?

People request changes to the domain by sending commands. They are named with a verb in the imperative mood plus and may include the aggregate type, for example ConfirmOrder. Unlike an event, a command is not a statement of fact; it's only a request, and thus may be refused. (A typical way to convey refusal is to throw an exception).


PATCH/PUT/POST/DELETE
    - thiscmd = eh.UUID()
    - CommandHandleCommand( thiscmd, PostCommand, BodyData )
        PostCommand RedfishAggregate command handler
            - Generate add/update events
            - generate event results event
    - Poll on redfishtreeprojector
        item.commandresults[thiscmd]


thermald sends TempChangeEvent -> redfish processor sends UpdateRedfishProperty Command -> Resource emits RedfishPropertyUpdated(data) --> redfish server --> (1) update internal model  (2) send redfish events


# address the following
    - methods per redfish resource
    - privileges per redfish resource  (example: password property)
        - user with configureuser can update username
        - password can only be updated with configureself or system privilege
    - need to be able to extract metadata from entire tree
    - actions:
        - able to specify action endpoints and the allowable parameters so we can formulate a command. This probably ends up being a separate set of "ActionCommand" command/response pairs, but need to be able to automatically generate the (validated?) action command from the core infrastructure.


PRIVILEGES
    - (DONE) TODO: create redfish resource must have required fields: id, context, type (DONE)
    - when redfishresourcecreated event happens, privileges saga goes and attaches privs. Look these up in the Privilege Registry based on entity.
        - what we store in the projected data doesn't need to match exactly with Privilege Registry schema, it can be optimized for our local processing
    - SAME for permissions. Look up permissions (read/write ability for each property) in the schema and set them. Also probably need to look in dell extended area somewhere.
    - write is only thing that cares about permissions
    - both get and setters care about privileges

METADATA
    - should be able to dynamically create metadata uri from the events as we register redfish resources

OUTPUT:
    map [type] manglingfunction
    When Get finds the resource, run it through the optional mangling function, if it exists
    Probably should have the generic output function be aware of per-property privileges

REGISTRY:
    - register URIs and mapping functions.
        - Commands look up URI in registry and apply additional transforms/checks
        - GET/PUT/PATCH/POST/etc use the registry to customize behaviours

QUESTION (COMPLETE):
    (DONE) how to figure out the CONFIGURE SELF problem?
    (NOPE) I think that for this, just match username against the name property (if either doesn't match, no property)
    (SOLUTION): When we set up login privs, if we see ConfigureSelf, change that to ConfigureSelf_${USERNAME}. When we attach privs to resources, if we see ConfigureSelf in the privilege map, change that to ConfigureSelf_${USERNAME}, based on the username listed in the resource.

BASIC AUTH
    - look up against the accounts in account service
    - look up role and then add privs

LOGIN session
    - logging in emits a create redfish resource command
    - logging out emits a remove redfish resource command
    - login creates a JWT token with privilege list
    - check the time against session timeout
    - session timeout emits the remove redfish resource command

PUT/PATCH/POST
    generic handler should check if-match or if-none-match headers
    concept: two types of handlers
        - How do we set up the handlers? HandleMethod property?
        - Generic: 
            operate directly on the aggregates. 
            Emit commands to update properties, 
            know how to read schema to see which fields should be updatable
            side effects outside the redfish tree can hang off of the events
        - specific
             (update this with REGISTRY idea, above)
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
    Redfish Resource

Commands:
    CreateRedfishResource  -  resourceURI
    AddRedfishResourceProperty
        - "@odata.id"
        - Members:
    UpdateRedfishResourceProperty
    RemoveRedfishResourceProperty
    CreateCollection
    AddCollectionMember
    RemoveCollectionMember

Events
    RedfishCreated
    RedfishPropertyAdded
    RedfishPropertyUpdated
    RedfishPropertyRemoved

Exceptions:
    RedfishAlreadyExists
    PropertyAlreadyExists
    CollectionAlreadyExists
    RedfishDoesntExist
    PropertyDoesntExist
    CollectionDoesntExist
