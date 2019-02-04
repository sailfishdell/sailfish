package domain

import (
	"context"
	"fmt"
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

// Optimized return
type EventChanUser interface {
	UseEventChan(chan<- eh.Event)
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
	BLANK = 0
	EQ    = 1
	LT    = 2
	GT    = 3
	GE    = 4
	LE    = 5
)

var comparisonString = map[int]string{
	BLANK: "",
	EQ:    " eq ",
	LT:    " lt ",
	GT:    " gt ",
	GE:    " ge ",
	LE:    " le ",
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

	auth := &RedfishAuthorizationProperty{UserName: rh.UserName, Privileges: rh.Privileges, Licenses: rh.d.GetLicenses(), Query: r.URL.Query()}

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

	directReturnChan := make(chan eh.Event, 1)
	if t, ok := cmd.(EventChanUser); ok {
		t.UseEventChan(directReturnChan)
	}

	ctx := WithRequestID(context.Background(), cmdID)
	if err := rh.d.CommandHandler.HandleCommand(ctx, cmd); err != nil {
		http.Error(w, "redfish handler could not handle command (type: "+string(cmd.CommandType())+"): "+err.Error(), http.StatusBadRequest)
		return
	}

	type eventRequiresCompletion interface {
		Done()
	}

	var event eh.Event
	select {
	case event = <-l.Inbox():
		// have to mark the event complete if we don't use Wait and take it off the bus ourselves
		if evtS, ok := event.(eventRequiresCompletion); ok {
			evtS.Done()
		}
	case <-reqCtx.Done():
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	case event = <-directReturnChan:
	}

	data, ok := event.Data().(*HTTPCmdProcessedData)
	if !ok {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Method == "GET" {
		// $top, $skip, $filter
		queryPresent := r.URL.Query().Get("$select") != "" || r.URL.Query().Get("$skip") != "" || r.URL.Query().Get("$top") != "" || r.URL.Query().Get("$filter") != ""
		// copy data before we slice and dice
		if queryPresent {
			mapstrint, ok := data.Results.(map[string]interface{})
			if ok {
				temp, err := DeepCopyMap(mapstrint)
				if err == nil {
					data.Results = temp
					data = handleCollectionQueryOptions(r, data)
					data = handleExpand(r, data)
					data = handleSelect(r, data)
				} else {
					fmt.Printf("SHOULD NOT HAPPEN, need to fix (talk to MEB): %s\n", err)
				}
			}
		}

		// TODO: Implementation shall return the 501, Not Implemented, status code for any query parameters starting with "$" that are not supported, and should return an extended error indicating the requested query parameter(s) not supported for this resource.
		// Implementation: for loop over the query parameters and check for anything unexpected
	}

	// set headers first
	w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains") // for A+ SSL Labs score
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("OData-Version", "4.0")
	w.Header().Set("Server", "sailfish")

	for k, v := range data.Headers {
		w.Header().Add(k, v)
	}

	if data.StatusCode != 0 {
		w.WriteHeader(data.StatusCode)
	}

	addEtag(w, data)

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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.Write(b)
	// END

	return
}

func getResourceEtag(ctx context.Context, agg *RedfishResourceAggregate, auth *RedfishAuthorizationProperty) string {
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
		etagprocessedintf, _ := ProcessGET(ctx, t, auth)
		etagstr, ok = etagprocessedintf.(string)
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

func createFilterArray(filter string) ([]FilterTest, bool) {
	filterArray := []FilterTest{}
	//TODO Right now only working with filter 'and' filter, 'or' is a whole 'nother ballgame
	splitFilter := strings.Split(filter, " and ")
	//For whatever filters may have been found, parse them out into a structure we can use
	for i := 0; i < len(splitFilter); i += 1 {
		tok := splitFilter[i]
		//Have a 'token' get the parts of it
		searchTok := BLANK
		for k := EQ; k <= LE; k += 1 {
			if strings.Contains(tok, comparisonString[k]) {
				searchTok = k
				break
			}
		}
		if searchTok != BLANK {
			subSplit := strings.Split(tok, comparisonString[searchTok])
			if strings.Contains(subSplit[0], "MessageID") {
				subSplit[0] = "MessageId" //Bug fix to Handle MSM
			}
			filterArray = append(filterArray, FilterTest{subSplit[0], subSplit[1], searchTok})
		} else {
			//Filter syntax violation
			return []FilterTest{}, false
		}
	}
	return filterArray, true
}

func processFilterOneObject(memberInstance map[string]interface{}, filterArray []FilterTest) bool {

	for j := 0; j < len(filterArray); j += 1 {
		//Find the String object we're trying to match
		var currentMember interface{}
		if memberInstance[filterArray[j].Category] != nil {
			currentMember = memberInstance[filterArray[j].Category]
		} else {
			//Drill down a layer further
			currentSubMember := memberInstance["Oem"].(map[string]interface{})
			if currentSubMember != nil {
				currentSubSubMember := currentSubMember["Dell"].(map[string]interface{})
				if currentSubSubMember[filterArray[j].Category] != nil {
					currentMember = currentSubSubMember[filterArray[j].Category]
				} else {
					return false
				}
			} else {
				return false
			}
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
			} else {
				//Other strings just get this as the keepElement
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
		case []string:
			//For this one, assume we're not going to find it until proven wrong
			//If we drop out of the loop without finding it, the we don't keep it
			for b := 0; b < len(localMember) && !keepElement; b += 1 {
				keepElement = strings.Contains(localMember[b], filterArray[j].SearchTerm)
			}
		//========= Time members =======
		case time.Time:
			//Handle time comparisons
			memberTime := localMember
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

func handleCollectionFilterMap(filter string, membersArr []map[string]interface{}) ([]map[string]interface{}, bool) {
	filterArray, ok := createFilterArray(filter)
	//If filter violation return nothing
	//If empty filters return everything
	if !ok {
		return []map[string]interface{}{}, ok
	}
	if len(filterArray) == 0 {
		return membersArr, ok
	}
	keepArray := []int{}

	//For each element in the log array apply all filters
	for i := 0; i < len(membersArr); i += 1 {
		memberInstance := membersArr[i]

		//Invert logic to determine delete
		if processFilterOneObject(memberInstance, filterArray) {
			keepArray = append(keepArray, i)
		}
	}

	//We have all logs we want to keep in an array, put those in our output
	returnArr := []map[string]interface{}{}
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

func handleCollectionQueryOptions(r *http.Request, d *HTTPCmdProcessedData) *HTTPCmdProcessedData {
	// the following query parameters affect how we return collections:
	skip := r.URL.Query().Get("$skip")
	top := r.URL.Query().Get("$top")
	filter := r.URL.Query().Get("$filter")
	res, ok := d.Results.(map[string]interface{})
	if !ok {
		// can't be a collection if it's not a map[string]interface{} (or, rather, we can't handle it here and would need to completely re-do this with introspection.)
		return d
	}

	members, ok := res["Members"]
	if !ok {
		return d
	}

	switch membersArr := members.(type) {
	case []interface{}:
		if filter != "" {
			membersArr, _ = handleCollectionFilter(filter, membersArr)
			//TODO handle bad filter request with HTTP error
			res["Members"] = membersArr
			res["Members@odata.count"] = len(membersArr)
		}
		var skipI int
		if skip != "" {
			tmpskipI, err := strconv.Atoi(skip)
			skipI = tmpskipI
			// TODO: http error on invalid skip request
			if err == nil && skipI > 0 {
				if skipI < len(membersArr) {
					// slice off the number we are supposed to skip from the beginning
					membersArr = membersArr[skipI:]
				} else {
					//Handle too big of a skip, so we don't fault
					membersArr = []interface{}{}
				}
				res["Members"] = membersArr
			}
		}

		topI := 50 //Default value so the Redfish output doesn't grow wild
		if top != "" {
			tmptopI, err := strconv.Atoi(top)
			// TODO: http error on invalid top request
			if err == nil {
				topI = tmptopI
			} else {
				topI = 0
			}
		}

		if topI > 0 && topI < len(membersArr) {
			// top means return exactly that many (or fewer), so slice off the end
			membersArr = membersArr[:topI]
			res["Members"] = membersArr

			// since we sliced off the end, add a nextlink user can follow to
			// get the rest (per redfish spec)
			// we'll be nice and preserve all the original query options
			q := r.URL.Query()
			q.Set("$skip", strconv.Itoa(skipI+topI))
			if top == "" {
				q.Add("$top", "50")
			}
			nextlink := url.URL{Path: r.URL.Path, RawQuery: q.Encode()}
			res["Members@odata.nextlink"] = nextlink.String()
		}
		return d
	case []map[string]interface{}:
		if filter != "" {
			membersArr, _ = handleCollectionFilterMap(filter, membersArr)
			//TODO handle bad filter request with HTTP error
			res["Members"] = membersArr
			res["Members@odata.count"] = len(membersArr)
		}

		var skipI int
		if skip != "" {
			tmpskipI, err := strconv.Atoi(skip)
			skipI = tmpskipI
			// TODO: http error on invalid skip request
			if err == nil && skipI > 0 {
				if skipI < len(membersArr) {
					// slice off the number we are supposed to skip from the beginning
					membersArr = membersArr[skipI:]
				} else {
					//Handle too big of a skip, so we don't fault
					membersArr = []map[string]interface{}{}
				}
				res["Members"] = membersArr
			}
		}

		topI := 50 //Default value so the Redfish output doesn't grow wild
		if top != "" {
			tmptopI, err := strconv.Atoi(top)
			// TODO: http error on invalid top request
			if err == nil {
				topI = tmptopI
			} else {
				topI = 0
			}
		}

		if topI > 0 && topI < len(membersArr) {
			// top means return exactly that many (or fewer), so slice off the end
			membersArr = membersArr[:topI]
			res["Members"] = membersArr

			// since we sliced off the end, add a nextlink user can follow to
			// get the rest (per redfish spec)
			// we'll be nice and preserve all the original query options
			q := r.URL.Query()
			q.Set("$skip", strconv.Itoa(skipI+topI))
			if top == "" {
				q.Add("$top", "50")
			}
			nextlink := url.URL{Path: r.URL.Path, RawQuery: q.Encode()}
			res["Members@odata.nextlink"] = nextlink.String()
		}
		return d
	default:
		//TODO Don't know how to handle this, what other are there?
		return d
	}

}

func handleExpand(r *http.Request, d *HTTPCmdProcessedData) *HTTPCmdProcessedData {
	//expand = r.URL.Query().Get("$expand")
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
		}
	}

	res, ok := d.Results.(map[string]interface{})
	if !ok {
		return d
	}

	trimSelect(res, selectQuery)

	return d
}

//TODO: regex still matches more than it should be matching
func trimSelect(r interface{}, selAry [][]Matcher) {

	//fmt.Printf("TRIMMING: r: %s, s: %s\n", r, selAry)
	res, ok := r.(map[string]interface{})
	if !ok {
		//fmt.Printf("Could not trim no map[string]interface{} item: %s = %T\n", r, r)
		return
	}

	for k, _ := range res {
		newQuery := [][]Matcher{}
		recurse := true
		//fmt.Printf("Check key %s\n", k)
		found := false
		if strings.HasPrefix(k, "@") {
			found = true
			recurse = false
		}
		if !found {
			for _, n := range selAry {
				//fmt.Printf("  check key %s with matcher %s\n", k, n[0])
				if n[0].MatchString(k) {
					found = true
					//fmt.Printf("\tfound\n")
					if len(n) <= 1 {
						recurse = false
					}
					newQuery = append(newQuery, n[1:])
				}
			}
		}
		if !found {
			delete(res, k)
		}
		if found && recurse {
			//fmt.Printf("=============subtrim start\n")
			trimSelect(res[k], newQuery)
			//fmt.Printf("=============subtrim end\n")
		}
	}

	return
}
