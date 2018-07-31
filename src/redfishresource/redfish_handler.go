package domain

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	eh "github.com/looplab/eventhorizon"
	log "github.com/superchalupa/go-redfish/src/log"
)

// CmdIDSetter interface is for commands that can take a given command id
type CmdIDSetter interface {
	SetCmdID(eh.UUID)
}

// AggIDSetter interface is for commands that run against a given aggregate
type AggIDSetter interface {
	SetAggID(eh.UUID)
}

// UserDetailsSetter is the interface that commands should implement to tell the handler if they handle authorization or std code should do it.
type UserDetailsSetter interface {
	SetUserDetails(string, []string) string
	// return codes:
	//      "checkMaster" - command check passed, but also check master
	//      "authorized"  - command check passed, go right ahead, no master check
	//      "unauthorized" - failed auth check in command, no need to check further

}

// HTTPParser is the interface for commands that want to do their own http body parsing
type HTTPParser interface {
	ParseHTTPRequest(*http.Request) error
}

// NewRedfishHandler is the constructor that returns a new RedfishHandler object.
func NewRedfishHandler(dobjs *DomainObjects, logger log.Logger, u string, p []string) *RedfishHandler {
	return &RedfishHandler{UserName: u, Privileges: p, d: dobjs, logger: logger}
}

// RedfishHandler is the container object that holds authorization information as well as domain objects.
type RedfishHandler struct {
	UserName   string
	Privileges []string
	d          *DomainObjects
	logger     log.Logger
}

func (rh *RedfishHandler) isAuthorized(requiredPrivs []string) (authorized string) {
	authorized = "unauthorized"
	if requiredPrivs == nil {
		requiredPrivs = []string{}
	}
outer:
	for _, p := range rh.Privileges {
		for _, q := range requiredPrivs {
			if p == q {
				authorized = "authorized"
				break outer
			}
		}
	}
	return
}

// TODO: need to write middleware that would allow different types of encoding on output
func (rh *RedfishHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Each command needs a unique UUID. We'll use that to listen for the HTTPProcessed Event, which should have a matching UUID.
	cmdID := eh.NewUUID()
	reqCtx := WithRequestID(r.Context(), cmdID)

	// All operations have to be on URLs that exist, so look it up in the tree
	aggID, ok := rh.d.GetAggregateIDOK(r.URL.Path)
	if !ok {
		http.Error(w, "Could not find URL: "+r.URL.Path, http.StatusNotFound)
		return
	}

	search := []eh.CommandType{}

	// load the aggregate for the URL we are operating on
	agg, err := rh.d.AggregateStore.Load(reqCtx, AggregateType, aggID)
	// type assertion to get real aggregate
	redfishResource, ok := agg.(*RedfishResourceAggregate)
	if ok {
		// prepend the plugins to the search path
		search = append(search, eh.CommandType(redfishResource.ResourceURI+":"+r.Method))
		search = append(search, eh.CommandType(redfishResource.Plugin+":"+r.Method))
	}
	search = append(search, eh.CommandType("http:RedfishResource:"+r.Method))

	// search through the commands until we find one that exists
	var cmd eh.Command
	for _, cmdType := range search {
		cmd, err = eh.CreateCommand(cmdType)
		if err == nil {
			break
		}
	}

	// with a proper error if we couldnt create a command of any kind
	if cmd == nil {
		http.Error(w, "could not create command", http.StatusBadRequest)
		return
	}

	// some optional interfaces that the commands might implement
	if t, ok := cmd.(CmdIDSetter); ok {
		t.SetCmdID(cmdID)
	}
	if t, ok := cmd.(AggIDSetter); ok {
		t.SetAggID(aggID)
	}

	// Choices: command can process Authorization, or we can process authorization, or both
	// If command implements UserDetailsSetter interface, we'll go ahead and call that.
	// Return code from command determines if we also check privs here
	authAction := "checkMaster"
	var implementsAuthorization bool
	if t, implementsAuthorization := cmd.(UserDetailsSetter); implementsAuthorization {
		authAction = t.SetUserDetails(rh.UserName, rh.Privileges)
	}
	// if command does not implement userdetails setter, we always check privs here
	if !implementsAuthorization || authAction == "checkMaster" {
		privsToCheck := redfishResource.PrivilegeMap[r.Method]

		// convert Privileges from []interface{} to []string (way more code than there should be for something this simple)
		var t []string
		switch privs := privsToCheck.(type) {
		case []string:
			t = append(t, privs...)
		case []interface{}:
			for _, v := range privs {
				if a, ok := v.(string); ok {
					t = append(t, a)
				}
			}
		default:
		}

		authAction = rh.isAuthorized(t)
	}

	if authAction != "authorized" {
		http.Error(w, "Not authorized to access this resource: ", http.StatusUnauthorized)
		return
	}

	// to avoid races, set up our listener first
	l, err := rh.d.HTTPWaiter.Listen(reqCtx, func(event eh.Event) bool {
		if event.EventType() != HTTPCmdProcessed {
			return false
		}
		if data, ok := event.Data().(*HTTPCmdProcessedData); ok {
			if data.CommandID == cmdID {
				return true
			}
		}
		return false
	})
	if err != nil {
		http.Error(w, "could not create waiter"+err.Error(), http.StatusInternalServerError)
		return
	}
	l.Name = "Redfish HTTP Listener"
	defer l.Close()

	// don't run parse until after privilege checks have been done
	defer r.Body.Close()
	if t, ok := cmd.(HTTPParser); ok {
		err := t.ParseHTTPRequest(r)
		if err != nil {
			http.Error(w, "Problems parsing http request: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	ctx := WithRequestID(context.Background(), cmdID)
	if err := rh.d.CommandHandler.HandleCommand(ctx, cmd); err != nil {
		http.Error(w, "redfish handler could not handle command (type: "+string(cmd.CommandType())+"): "+err.Error(), http.StatusBadRequest)
		return
	}

	event, err := l.Wait(reqCtx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data, ok := event.Data().(*HTTPCmdProcessedData)
	if !ok {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Method == "GET" {
		// $top, $skip, $filter
		data = handleCollectionQueryOptions(r, data)
		data = handleExpand(r, data)
		data = handleSelect(r, data)

		// TODO: Implementation shall return the 501, Not Implemented, status code for any query parameters starting with "$" that are not supported, and should return an extended error indicating the requested query parameter(s) not supported for this resource.
		// Implementation: for loop over the query parameters and check for anything unexpected
	}

	// set headers first
	w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains") // for A+ SSL Labs score
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("OData-Version", "4.0")
	w.Header().Set("Server", "go-redfish")

	for k, v := range data.Headers {
		w.Header().Add(k, v)
	}

	/*
	       // START
	       // STREAMING ENCODE TO OUTPUT (not possible to get content length)
	   	// and then encode response
	   	enc := json.NewEncoder(w)
	   	// enc.SetIndent("", "  ")
	   	enc.Encode(data.Results)
	       // END
	*/

	// START:
	// uses more ram: encode to buffer first, get length, then send
	// This  lets 'ab' (apachebench) properly do keepalives
	b, err := json.Marshal(data.Results)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.Write(b)
	// END

	return
}

func handleExpand(r *http.Request, d *HTTPCmdProcessedData) *HTTPCmdProcessedData {
	//expand = r.URL.Query().Get("$expand")
	return d
}

func handleSelect(r *http.Request, d *HTTPCmdProcessedData) *HTTPCmdProcessedData {
	//sel = r.URL.Query().Get("$select")
	return d
}

func handleCollectionQueryOptions(r *http.Request, d *HTTPCmdProcessedData) *HTTPCmdProcessedData {
	// the following query parameters affect how we return collections:
	//skip = r.URL.Query().Get("$skip")
	//top = r.URL.Query().Get("$top")
	//filter = r.URL.Query().Get("$filter")

	return d
}
