package domain

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/json-iterator/go"

	eh "github.com/looplab/eventhorizon"
	log "github.com/superchalupa/sailfish/src/log"
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
	SetUserDetails(*RedfishAuthorizationProperty) string
	// return codes:
	//		"checkMaster" - command check passed, but also check master
	//		"authorized"  - command check passed, go right ahead, no master check
	//		"unauthorized" - failed auth check in command, no need to check further

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

func (rh *RedfishHandler) verifyLocationURL(reqCtx context.Context, url string) bool {

	// check the existance early to avoid setting up listener.
	_, ok := rh.d.GetAggregateIDOK(url)
	if ok {
		// URL exists so just return
		rh.logger.Debug("Location exists", "URL", url)
		return ok
	}

	location_timeout := 10
	ctx, cancel := context.WithTimeout(reqCtx, time.Duration(location_timeout)*time.Second)
	defer cancel()

	// to avoid races, set up our listener first
	listener, err := rh.d.EventWaiter.Listen(ctx, func(event eh.Event) bool {
		if event.EventType() == RedfishResourceCreated {
			if data, ok := event.Data().(*RedfishResourceCreatedData); ok {
				if data.ResourceURI == url {
					rh.logger.Debug("Location created", "URI", data.ResourceURI)
					return true
				}
			}
			return false
		}
		return false
	})
	if err != nil {
		rh.logger.Error("could not create waiter for location", "err", err.Error(), "url", url)
		return false
	}
	listener.Name = "location update listener"
	defer listener.Close()

	// make sure we didn't miss the event while creating the listner
	_, ok = rh.d.GetAggregateIDOK(url)
	if ok {
		// URL exists so just return
		rh.logger.Debug("Location exists", "URL", url)
		return ok
	}

	rh.logger.Warn("Location does not exist, wait for it", "URL", url)
	_, err = listener.Wait(ctx)
	if err != nil {
		rh.logger.Error("location wait timed out", "URI", url)
		return false
	}

	// location was created
	return true
}

// TODO: need to write middleware that would allow different types of encoding on output
func (rh *RedfishHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Each command needs a unique UUID. We'll use that to listen for the HTTPProcessed Event, which should have a matching UUID.
	cmdID := eh.NewUUID()
	reqCtx := WithRequestID(r.Context(), cmdID)

	// All operations have to be on URLs that exist, so look it up in the tree
	aggID, ok := rh.d.GetAggregateIDOK(r.URL.Path)
	if !ok {
		rh.logger.Warn("Could not find URL", "url", r.URL.Path)
		http.Error(w, "Could not find URL: "+r.URL.Path, http.StatusNotFound)
		return
	}

	search := make([]eh.CommandType, 0, 3) // 3 == max number of paths listed below, change if we add more

	// load the aggregate for the URL we are operating on
	agg, err := rh.d.AggregateStore.Load(reqCtx, AggregateType, aggID)
	// type assertion to get real aggregate
	redfishResource, ok := agg.(*RedfishResourceAggregate)
	if ok {
		// prepend the plugins to the search path
		search = append(search, eh.CommandType(redfishResource.ResourceURI+":"+r.Method)) // preallocated
		search = append(search, eh.CommandType(redfishResource.Plugin+":"+r.Method))      // preallocated
	}
	// short version - save memory
	search = append(search, eh.CommandType("R:"+r.Method)) // preallocated
	// long version for backwards compat (old style)
	search = append(search, eh.CommandType("http:RedfishResource:"+r.Method)) // preallocated

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
		rh.logger.Warn("could not create command", "url", r.URL.Path)
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

	// authorization, and query data collected here
	auth := rh.SetupAuthorization(r, redfishResource)

	// Choices: command can process Authorization, or we can process authorization, or both
	// If command implements UserDetailsSetter interface, we'll go ahead and call that.
	// Return code from command determines if we also check privs here
	authAction := "checkMaster"
	var implementsAuthorization bool
	if t, implementsAuthorization := cmd.(UserDetailsSetter); implementsAuthorization {
		authAction = t.SetUserDetails(auth)
	}
	// if command does not implement userdetails setter, we always check privs here
	if !implementsAuthorization || authAction == "checkMaster" {
		privsToCheck := redfishResource.PrivilegeMap[MapStringToHTTPReq(r.Method)]

		// convert Privileges from []interface{} to []string (way more code than there should be for something this simple)
		var t []string
		switch privs := privsToCheck.(type) {
		case []string:
			t = make([]string, 0, len(privs))
			t = append(t, privs...) // preallocated
		case []interface{}:
			t = make([]string, 0, len(privs))
			for _, v := range privs {
				if a, ok := v.(string); ok {
					t = append(t, a) // preallocated
				}
			}
		default:
			t = make([]string, 0, 0)
		}

		authAction = rh.isAuthorized(t)
	}

	if authAction != "authorized" {
		rh.logger.Warn("Not authorized to access this resource.", "url", r.URL.Path)
		http.Error(w, "Not authorized to access this resource: ", http.StatusMethodNotAllowed)
		return
	}

	// for intial implementation of etags, we will check etags right here. we may need to move this around later. For example, the command might need to handle it
	// TODO: this all has to happen after the privilege check
	if match := r.Header.Get("If-None-Match"); match != "" {
		e := getResourceEtag(reqCtx, redfishResource, auth)
		if e != "" {
			if match == e {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}

	// TODO: If-Match must be able to match comma separated list
	if match := r.Header.Get("If-Match"); match != "" {
		e := getResourceEtag(reqCtx, redfishResource, auth)
		if e != "" {
			if match != e {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
		}
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
		rh.logger.Warn("could not create waiter", "err", err.Error(), "url", r.URL.Path)
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
			rh.logger.Warn("Problems parsing http request: ", "err", err.Error(), "url", r.URL.Path)
			http.Error(w, "Problems parsing http request: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	ctx := WithRequestID(context.Background(), cmdID)

	if err := rh.d.CommandHandler.HandleCommand(ctx, cmd); err != nil {
		rh.logger.Warn("redfish handler could not handle command", "type", string(cmd.CommandType()), "err", err.Error(), "url", r.URL.Path, "resource", redfishResource, "cmd", cmd)
		http.Error(w, "redfish handler could not handle command (type: "+string(cmd.CommandType())+"): "+err.Error(), http.StatusBadRequest)
		return
	}

	type eventRequiresCompletion interface {
		Done()
	}

	var event eh.Event
	select {
	case event = <-l.Inbox():
	case <-reqCtx.Done():
		rh.logger.Warn("Request cancelled, aborting http response", "url", r.URL.Path)
		http.Error(w, "Request cancelled, aborting http response", http.StatusInternalServerError)
		return
	}

	// have to mark the event complete if we don't use Wait and take it off the bus ourselves
	if evtS, ok := event.(eventRequiresCompletion); ok {
		evtS.Done()
	}

	data, ok := event.Data().(*HTTPCmdProcessedData)
	if !ok {
		rh.logger.Warn("Did not get an HTTPCmdProcessedData event, that's wierd.", "url", r.URL.Path, "event", event.Data())
		http.Error(w, "Did not get an HTTPCmdProcessedData event, that's wierd", http.StatusInternalServerError)
		return
	}

	// filter redfish data
	rh.DoFilter(auth, data)

	// set headers first
	w.Header().Set("OData-Version", "4.0")
	w.Header().Set("Server", "sailfish")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Cache-Control", "no-Store,no-Cache")
	w.Header().Set("Pragma", "no-cache")

	// security headers
	w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains") // for A+ SSL Labs score
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("X-Content-Security-Policy", "default-src 'self'")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// compatibility headers
	w.Header().Set("X-UA-Compatible", "IE=11")

	addEtag(w, data)

	// check if k has 'location' and if so check if URI exists, and if not add a new listener
	// and wait for it to show up
	for k, v := range data.Headers {
		if strings.EqualFold(k, "location") {
			rh.verifyLocationURL(reqCtx, v)
		}

		w.Header().Add(k, v)
	}

	if data.StatusCode != 0 {
		w.WriteHeader(data.StatusCode)
	}

	/*
		   // START
		   // STREAMING ENCODE TO OUTPUT (not possible to get content length)
		// and then encode response
		enc := json.NewEncoder(w)
		// enc.SetIndent("", "	")
		enc.Encode(data.Results)
		   // END
	*/

	// START:
	// uses more ram: encode to buffer first, get length, then send
	// This	 lets 'ab' (apachebench) properly do keepalives

	// stdlib marshaller (slower?)
	// must import "encoding/json"
	// b, err := json.Marshal(data.Results)

	// faster json marshal:
	//var json = jsoniter.ConfigCompatibleWithStandardLibrary
	var json = jsoniter.ConfigFastest
	b, err := json.Marshal(data.Results)

	if err != nil {
		rh.logger.Warn("Error encoding JSON for output: ", "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.Write(b)
	// END

	redfishResource.Lock()
	if redfishResource.access == nil {
		redfishResource.access = map[HTTPReqType]time.Time{}
	}
	redfishResource.access[MapStringToHTTPReq(r.Method)] = time.Now()
	redfishResource.Unlock()

	return
}

func getResourceEtag(ctx context.Context, agg *RedfishResourceAggregate, auth *RedfishAuthorizationProperty) string {
	agg.Lock()
	defer agg.Unlock()

	v := agg.Properties.Value
	m, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}

	etagintf, ok := m["@odata.etag"]
	if !ok {
		return ""
	}

	var etagstr string

	switch t := etagintf.(type) {
	case *RedfishResourceProperty:
		NewGet(ctx, agg, t, auth)
		etagIntf := Flatten(t, false)
		etagstr, ok = etagIntf.(string)
		if !ok {
			return ""
		}

	case string:
		etagstr = t

	default:
	}

	return etagstr
}

func addEtag(w http.ResponseWriter, d *HTTPCmdProcessedData) *HTTPCmdProcessedData {
	res, ok := d.Results.(map[string]interface{})
	if !ok {
		// no way it has an etag, return
		return d
	}

	etag, ok := res["@odata.etag"]
	if !ok {
		// no etag
		return d
	}

	etagStr, ok := etag.(string)
	if ok {
		w.Header().Add("Etag", etagStr)
	}

	return d
}
