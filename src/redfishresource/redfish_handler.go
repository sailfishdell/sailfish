package domain

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/json-iterator/go"

	eh "github.com/looplab/eventhorizon"
	log "github.com/superchalupa/sailfish/src/log"

	"regexp"
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

const (
	BLANK    = 0
	EQ       = 1
	LT       = 2
	GT       = 3
	GE       = 4
	LE       = 5
	CONTAINS = 6
)

var comparisonString = map[int]string{
	BLANK:    "",
	EQ:       " eq ",
	LT:       " lt ",
	GT:       " gt ",
	GE:       " ge ",
	LE:       " le ",
	CONTAINS: "contains",
}

var sevInteger = map[string]int{
	"Fatal":    4,
	"Critical": 3,
	"Warning":  2,
	"OK":       1,
}

type FilterTest struct {
	Category   string //What field we compare against
	SearchTerm string //For string search the second term
	Comparator int    //Operator ==, <, >, >=, <=
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

	auth := &RedfishAuthorizationProperty{
		UserName:   rh.UserName,
		Privileges: rh.Privileges,
		Licenses:   rh.d.GetLicenses(),
		Query:      r.URL.Query(),
	}

	// default to top=50 to reduce cpu
	auth.top = 50
	auth.doTop = true
	if tstr := r.URL.Query().Get("$top"); tstr != "" {
		auth.top, err = strconv.Atoi(tstr)
		auth.doTop = (err == nil)
	}

	if tstr := r.URL.Query().Get("$skip"); tstr != "" {
		auth.skip, err = strconv.Atoi(tstr)
		auth.doSkip = (err == nil)
	}

	if tstr := r.URL.Query().Get("$filter"); tstr != "" {
		auth.filter = r.URL.Query().Get("$filter")
		auth.doFilter = true
	}

	if tstr := r.URL.Query().Get("$select"); tstr != "" {
		auth.sel = r.URL.Query()["$select"]
		auth.doSel = true
	}

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

	if r.Method == "GET" {
		if auth.doSkip || auth.doTop || auth.doFilter {
			data = handleCollectionQueryOptions(r, auth, data)
		}
		if auth.doSel {
			data = handleSelect(r, data)
		}

		// TODO: Implementation shall return the 501, Not Implemented, status code for any query parameters starting with "$" that are not supported, and should return an extended error indicating the requested query parameter(s) not supported for this resource.
		// Implementation: for loop over the query parameters and check for anything unexpected
	}

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

// input is a string with pattern ( val1, val2) or  [ val1, val2 ] or { val1, val2 }
// return is a list  [ val1, val2 ]
// quotes and spaces are trimmed from val1 and val2
func regexGetStrInParanth(val string) ([]string, bool) {
	m := make([]string, 2)
	re := regexp.MustCompile(`[\(\[\{](.*)\,(.*)[\)\]\}]`)

	matches := re.FindStringSubmatch(val)
	if len(matches) == 0 {
		return m, false
	}

	val1 := matches[1]
	val2 := matches[2]

	//cleanup
	val1 = strings.TrimSpace(val1)
	val2 = strings.TrimSpace(val2)

	re = regexp.MustCompile(`^['"].*['"]$`)
	lenStr := len(val1)
	if re.FindString(val1) != "" {
		val1 = val1[1 : lenStr-1]
	}

	lenStr = len(val2)
	if re.FindString(val2) != "" {
		val2 = val2[1 : lenStr-1]
	}

	m[0] = val1
	m[1] = val2

	return m, true

}

// takes a url filter and organizes it in a list of structures
func createFilterArray(filter string) ([]FilterTest, bool) {
	filterArray := []FilterTest{}
	//TODO Right now only working with filter 'and' filter, 'or' is a whole 'nother ballgame
	splitFilter := strings.Split(filter, " and ")
	//For whatever filters may have been found, parse them out into a structure we can use
	// iterates list providing the index and element
	for _, tok := range splitFilter {
		//Have a 'token' get the parts of it
		searchTok := BLANK
		for k := EQ; k <= CONTAINS; k += 1 {
			if strings.Contains(tok, comparisonString[k]) {
				searchTok = k
				break
			}
		}
		if searchTok == CONTAINS {
			// matches (<string>, <string>)
			subSplit, ok := regexGetStrInParanth(tok)
			if ok {
				filterArray = append(filterArray, FilterTest{subSplit[0], subSplit[1], CONTAINS})
			}

		} else if searchTok != BLANK {
			subSplit := strings.Split(tok, comparisonString[searchTok])
			if strings.Contains(subSplit[0], "MessageID") {
				subSplit[0] = "MessageId" //Bug fix to Handle MSM
			}
			if subSplit[1] == "Ok" {
				subSplit[1] = "OK" //Bug fix to Handle MSM
			}
			filterArray = append(filterArray, FilterTest{subSplit[0], subSplit[1], searchTok})

		} else {
			//Filter syntax violation
			return []FilterTest{}, false
		}
	}
	return filterArray, true
}

// goes through a layered map (memberInstance) using the list (p) to find the final value
func getValueWithPath(memberInstance map[string]interface{}, p []string) (interface{}, bool) {
	var mVal interface{}
	var ok bool
	for i, v := range p {
		if i == len(p)-1 {
			mVal, ok = memberInstance[v].(interface{})
		} else {
			memberInstance, ok = memberInstance[v].(map[string]interface{})
		}

		// exit out early if value not retrieved
		if !ok {
			return mVal, ok
		}

	}

	return mVal, ok

}

// looks for string (c) in nested map (memberInstance)
func getCategoryValue(memberInstance map[string]interface{}, c string) (interface{}, bool) {
	//Find the  object we're trying to match
	ok := false
	var mVal interface{}

	// for filters providing a path to filter value (like faults)
	cL := strings.Split(c, "/")
	mVal, ok = getValueWithPath(memberInstance, cL)

	if ok {
		return mVal, true
	}

	// check if this is a log message filter
	log_pathL := []string{"Oem", "Dell", c}
	mVal, ok = getValueWithPath(memberInstance, log_pathL)

	return mVal, ok
}

// returns true if the memberInstance matches with the url filter, stored as filterArray
func processFilterOneObject(memberInstance map[string]interface{}, filterArray []FilterTest) bool {

	for j := 0; j < len(filterArray); j += 1 {
		currentMember, rc := getCategoryValue(memberInstance, filterArray[j].Category)
		if rc == false {
			return false
		}

		//Only keep item IF we find at least one term to search against
		//Reason for this is we keep only things that match all terms, and no match no keep
		keepElement := false
		//We could have multiple member types
		switch localMember := currentMember.(type) {
		//========= String members =======
		case string:
			if filterArray[j].Category == "Severity" {
				//Severity comparisons string to ENUM sorta
				memberSev := sevInteger[localMember]
				searchSev := sevInteger[filterArray[j].SearchTerm]
				if memberSev > 0 && memberSev < 5 && searchSev > 0 && searchSev < 5 {
					switch filterArray[j].Comparator {
					case GE:
						keepElement = memberSev >= searchSev
					case GT:
						keepElement = memberSev > searchSev
					case LE:
						keepElement = memberSev <= searchSev
					case LT:
						keepElement = memberSev < searchSev
					case EQ:
						keepElement = memberSev == searchSev
					}
				}

			} else if filterArray[j].Category == "Created" {
				//Handle time comparisons
				memberTime, err := time.Parse(time.RFC3339, localMember)
				if err != nil {
					//If there is no : in the time adjustment RFC3339 breaks
					memberTime, err = time.Parse("2006-01-02T15:04:05-0700", localMember)
				}

				//Use fallback custom parser because of bug in Go time RFC3339 implementation
				searchTime, err := time.Parse(time.RFC3339, filterArray[j].SearchTerm)
				if err != nil {
					//If there is no : in the time adjustment RFC3339 breaks
					searchTime, err = time.Parse("2006-01-02T15:04:05-0700", filterArray[j].SearchTerm)
				}
				if err == nil {
					//If we were able to parse the input time with one of the methods
					switch filterArray[j].Comparator {
					case GE:
						keepElement = memberTime.Equal(searchTime)
						if !keepElement {
							keepElement = memberTime.After(searchTime)
						}
					case GT:
						keepElement = memberTime.After(searchTime)
					case LE:
						keepElement = memberTime.Equal(searchTime)
						if !keepElement {
							keepElement = memberTime.Before(searchTime)
						}
					case LT:
						keepElement = memberTime.Before(searchTime)
					case EQ:
						keepElement = memberTime.Equal(searchTime)
					}
				}

			} else {
				//fmt.Println(localMember, filterArray[j].SearchTerm, strings.Contains(localMember, filterArray[j].SearchTerm))
				keepElement = strings.Contains(localMember, filterArray[j].SearchTerm)
			}
		//========= Integer members =======
		case int:
			//Currently no fields like this?
			searchInt, err := strconv.Atoi(filterArray[j].SearchTerm)
			if err == nil {
				switch filterArray[j].Comparator {
				case GE:
					keepElement = localMember >= searchInt
				case GT:
					keepElement = localMember > searchInt
				case LE:
					keepElement = localMember <= searchInt
				case LT:
					keepElement = localMember < searchInt
				case EQ:
					keepElement = localMember == searchInt
				}
			}
		//========= String array members =======
		case []interface{}:
			//For this one, assume we're not going to find it until proven wrong
			//If we drop out of the loop without finding it, the we don't keep it
			for b := 0; b < len(localMember) && !keepElement; b += 1 {
				localMemberStr := localMember[b].(string)
				keepElement = strings.Contains(localMemberStr, filterArray[j].SearchTerm)
			}
		//========= The what?! members =======
		default:
			//Most like map[string]interface{}, problem is we don't know how to drill down
			//Give it a pass
			keepElement = true
		}
		if !keepElement {
			return false
		}
	}
	//We get through the loop without any early outs, we have a success
	return true
}

func handleCollectionFilter(filter string, membersArr []interface{}) ([]interface{}, bool) {

	filterArray, ok := createFilterArray(filter)
	//If filter violation return nothing
	//If empty filters return everything
	if !ok {
		return []interface{}{}, ok
	}
	if len(filterArray) == 0 {
		return membersArr, ok
	}
	keepArray := []int{}

	//For each element in the log array apply all filters
	for i := 0; i < len(membersArr); i += 1 {
		memberInstance, ok := membersArr[i].(map[string]interface{})
		if !ok {
			return membersArr, ok
		}
		//Invert logic to determine delete
		if processFilterOneObject(memberInstance, filterArray) {
			keepArray = append(keepArray, i)
		}

	}

	//We have all logs we want to keep in an array, put those in our output
	returnArr := []interface{}{}
	//Special case to same some cycles, if we filtered out nothing in the end, just return the original
	if len(keepArray) == len(membersArr) {
		returnArr = membersArr
	} else {
		for _, index := range keepArray {
			returnArr = append(returnArr, membersArr[index])
		}
	}
	return returnArr, ok
}

func handleCollectionQueryOptions(r *http.Request, a *RedfishAuthorizationProperty, d *HTTPCmdProcessedData) *HTTPCmdProcessedData {
	res, ok := d.Results.(map[string]interface{})
	if !ok {
		// can't be a collection if it's not a map[string]interface{}
		return d
	}

	// make sure it is an actual collection and return if not
	members, ok := res["Members"]
	if !ok {
		return d
	}

	// then type assert to ensure it's an array
	membersArr, ok := members.([]interface{})
	if !ok {
		return d
	}

	// Need to make a one-level deep copy to not disturb the original data
	newResults := map[string]interface{}{}
	for k, v := range res {
		// skip 'members', that will be copied separately, next
		if k == "members" {
			continue
		}
		newResults[k] = v
	}

	if a.doFilter {
		//TODO handle bad filter request with HTTP error
		//
		// redfish standard says that filtering changes odata.count
		// but top and skip do not
		membersArr, _ = handleCollectionFilter(a.filter, membersArr)
	}
	//Always update count, sometimes it comes out wrong for some reason
	newResults["Members@odata.count"] = len(membersArr)

	// figure out parameters for the final slice
	beginning := 0
	end := len(membersArr)

	if a.doSkip && a.skip > 0 {
		if a.skip < len(membersArr) {
			beginning = a.skip
		} else {
			beginning = end
		}
	}

	if a.doTop && a.top > 0 {
		if a.top+beginning < len(membersArr) {
			end = beginning + a.top

			// since we sliced off the end, add a nextlink user can follow to
			// get the rest (per redfish spec)
			// we'll be nice and preserve all the original query options
			q := r.URL.Query()
			q.Set("$skip", strconv.Itoa(a.skip+a.top))
			q.Set("$top", strconv.Itoa(a.top))
			nextlink := url.URL{Path: r.URL.Path, RawQuery: q.Encode()}
			newResults["Members@odata.nextlink"] = nextlink.String()
		}
	}

	// so we are going to return pointer to the records from the original cached
	// array. Note that we should have a read lock on this data until it's
	// serialized to user, so it shouldn't change under us
	newResults["Members"] = membersArr[beginning:end]

	d.Results = newResults
	return d
}

type Matcher interface {
	MatchString(string) bool
}

func handleSelect(r *http.Request, d *HTTPCmdProcessedData) *HTTPCmdProcessedData {
	q := r.URL.Query()
	selAry, ok := q["$select"]
	if !ok {
		return d
	}

	// Leaving debug prints commented out because this is hairy and they'll be needed when we revisit this
	//fmt.Printf("SELECT: %s\n", selAry)

	makesel := func(q *[][]Matcher, s []string) {
		b := []Matcher{}
		if len(s) == 0 {
			return
		}
		for _, i := range s {
			re, err := regexp.Compile(strings.Replace(i, "*", ".*", -1))
			if err != nil {
				return
			}
			b = append(b, re)
		}
		*q = append(*q, b)
	}

	selectQuery := [][]Matcher{}
	for _, j := range selAry {
		for _, i := range strings.Split(j, ",") {
			makesel(&selectQuery, strings.Split(i, "/"))

			// WORKAROUND FOR BROKEN MSM
			makesel(&selectQuery, strings.Split("Attributes/"+i, "/"))
			makesel(&selectQuery, []string{"Id"})
			makesel(&selectQuery, []string{"Name"})
			makesel(&selectQuery, []string{"Description"})
			makesel(&selectQuery, []string{"@*"})
			makesel(&selectQuery, []string{"AttributeRegistry"})
		}
	}

	source := d.Results
	d.Results = map[string]interface{}{}

	copySelect(d.Results, source, selectQuery)

	return d
}

//TODO: regex still matches more than it should be matching
func copySelect(dest, src interface{}, selAry [][]Matcher) {
	srcM, ok := src.(map[string]interface{})
	if !ok {
		//fmt.Printf("NOT a MAP")
		dest = src
		return
	}

	destM, ok := dest.(map[string]interface{})
	if !ok {
		// can't happen!
		//fmt.Printf("CANT HAPPEN")
		dest = src
		return
	}

	for k, v := range srcM {
		newQuery := [][]Matcher{}
		recurse := true
		found := false
		if !found {
			for _, n := range selAry {
				if n[0].MatchString(k) {
					//fmt.Printf("FOUND")
					found = true
					if len(n) <= 1 {
						recurse = false
					}
					newQuery = append(newQuery, n[1:])
				}
			}
		}
		if found && recurse {
			destM[k] = map[string]interface{}{}
			copySelect(destM[k], srcM[k], newQuery)
		} else if found {
			destM[k] = v
		}
	}

	return
}
