package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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
	SetUserDetails(string, []string) string
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
	d		   *DomainObjects
	logger	   log.Logger
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
	//fmt.Printf("GET URL IS : '%s'\n", r.URL.Path)

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

	// for intial implementation of etags, we will check etags right here. we may need to move this around later. For example, the command might need to handle it
	// TODO: this all has to happen after the privilege check
	if match := r.Header.Get("If-None-Match"); match != "" {
		//fmt.Printf("GOT If-None-Match: '%s'\n", match)
		e := getResourceEtag(reqCtx, redfishResource)
		//fmt.Printf("\tetag: '%s'\n", e)
		if e != "" {
			if match == e {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}

	// TODO: If-Match must be able to match comma separated list
	if match := r.Header.Get("If-Match"); match != "" {
		//fmt.Printf("GOT If-Match: '%s'\n", match)
		e := getResourceEtag(reqCtx, redfishResource)
		//fmt.Printf("\tetag: '%s'\n", e)
		if e != "" {
			if match != e {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
		}
	}

	// add authorization details
	redfishResource.Authorization.UserName = rh.UserName
	redfishResource.Authorization.Privileges = rh.Privileges
	redfishResource.Authorization.Licenses = rh.d.GetLicenses()

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
		//fmt.Printf("Calling to process top,skip,filter\n")
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

func getResourceEtag(ctx context.Context, agg *RedfishResourceAggregate) string {
	//fmt.Printf("get etag\n")

	v := agg.Properties.Value
	m, ok := v.(map[string]interface{})
	if !ok {
		//fmt.Printf("not a map[string]interface{}\n")
		return ""
	}

	etagintf, ok := m["@odata.etag"]
	if !ok {
		//fmt.Printf("no @odata.etag\n")
		return ""
	}

	var etagstr string

	switch t := etagintf.(type) {
	case *RedfishResourceProperty:
		etagprocessedintf, _ := ProcessGET(ctx, t, &agg.Authorization)
		etagstr, ok = etagprocessedintf.(string)
		if !ok {
			//fmt.Printf("@odata.etag not a string: %T - %#v\n", etagprocessedintf, etagprocessedintf)
			return ""
		}
		//fmt.Printf("processed RedfishResourceProperty to string! yay\n")

	case string:
		etagstr = t
		//fmt.Printf("direct string")

	default:
		//fmt.Printf("unknown @odata.etag: %T - %#v\n", t, t)
	}

	//fmt.Printf("GOT ETAG: '%s'\n", etagstr)
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
func handleCollectionFilter(filter string, membersArr []interface{}) []interface{} {
	//fmt.Printf("filter: %s\n", filter)

	const (
		BLANK = 0
		EQ = 1
		LT = 2
		GT = 3
		GE = 4
		LE = 5
	)

	var comparisonString = map[int]string{
		BLANK:	"",
		EQ:		" eq ",
		LT:		" lt ",
		GT:		" gt ",
		GE:		" ge ",
		LE:		" le ",
	}

	var sevInteger = map[string]int{
		"Fatal":	4,
		"Critical":	3,
		"Warning":	2,
		"OK":		1,
	}

	type FilterTest struct {
		Category string //What field we compare against
		SearchTerm	 string	   //For string search the second term
		Comparator	   int	  //Operator ==, <, >, >=, <=
		Time   time.Time	//Time construct for date compare
	}

	filterArray := []FilterTest{}
	keepArray := []int{}
	//TODO Right now only working with filter 'and' filter, 'or' is a whole 'nother ballgame
	splitFilter := strings.Split(filter, " and ")
	//For whatever filters may have been found, parse them out into a structure we can use
	for i := 0 ; i < len(splitFilter) ; i += 1 {
		tok := splitFilter[i]
		//Have a 'token' get the parts of it
		searchTok := BLANK
		for k := EQ; k <= LE; k += 1 {
			//fmt.Printf("Checking for:%s\n",comparisonString[k])
			if strings.Contains(tok, comparisonString[k]){
				searchTok = k
				break
			}
		}
		if searchTok != BLANK {
			subSplit := strings.Split(tok, comparisonString[searchTok])
			tmpTime, _ := time.Parse("2006-01-02T15:04:05-0700", subSplit[1])
			if strings.Contains (subSplit[0], "MessageID") {
				subSplit[0] = "MessageId" //Bug fix? From which party?
			}
			filterArray = append(filterArray, FilterTest{subSplit[0], subSplit[1], searchTok, tmpTime})
			//fmt.Println(filterArray[len(filterArray)-1])
		}
	}

	//For each element in the log array apply all filters
	for i := 0; i < len(membersArr) ; i += 1 {
		memberInstance := membersArr[i].(map[string]interface{})
		keepArray = append(keepArray, i)
		//fmt.Println(currentMember)
		for j := 0; j < len(filterArray) ; j += 1 {
			//Each filter on the same object
			//fmt.Println(filterArray[j])
			var currentMember interface{}
			if memberInstance[filterArray[j].Category] != nil {
				currentMember = memberInstance[filterArray[j].Category]
			} else {
				//Drill down a layer further
				currentSubMember := memberInstance["Oem"].(map[string]interface{})
				if currentSubMember != nil {
					currentSubSubMember := currentSubMember["Dell"].(map[string]interface{})
					if currentSubSubMember[filterArray[j].Category] != nil {
						//fmt.Printf("Found %s in Oem:Dell\n", filterArray[j].Category)
						currentMember = currentSubSubMember[filterArray[j].Category]
					} else {
						//fmt.Printf("ERROR can't find %s\n", filterArray[j].Category)
						currentMember = nil
					}
				} else {
						//fmt.Printf("ERROR can't find %s\n", filterArray[j].Category)
						currentMember = nil
				}
			}
			if currentMember != nil {
				//fmt.Printf("Comparing %s%sto %s\n", filterArray[j].Category, comparisonString[filterArray[j].Comparator], filterArray[j].SearchTerm)
				if filterArray[j].Category == "Created" {
					//Handle time comparisons
					memberTime := currentMember.(time.Time)
					var result bool
					switch filterArray[j].Comparator {
						case GE:
							result = memberTime.Equal(filterArray[j].Time)
							if !result {
								result = memberTime.After(filterArray[j].Time)
							}
						case GT:
							result = memberTime.After(filterArray[j].Time)
						case LE:
							result = memberTime.Equal(filterArray[j].Time)
							if !result {
								result = memberTime.Before(filterArray[j].Time)
							}
						case LT:
							result = memberTime.Before(filterArray[j].Time)
						case EQ:
							result = memberTime.Equal(filterArray[j].Time)
						default :
							result = true //What happened?
					}
					//Invert logic to determine delete
					if !result {
						//Remove
						//fmt.Printf("Remove by time: %v\n", memberInstance)
						keepArray = keepArray[:len(keepArray)-1]
						break
					}
				} else if filterArray[j].Category == "Severity" {
					//Severity comparisons
					memberSev := sevInteger[currentMember.(string)]
					searchSev := sevInteger[filterArray[j].SearchTerm]
					if memberSev > 0 && memberSev < 5 && searchSev > 0 && searchSev < 5 {
						var result bool
						switch filterArray[j].Comparator {
							case GE:
								result = memberSev >= searchSev
							case GT:
								result = memberSev > searchSev
							case LE:
								result = memberSev <= searchSev
							case LT:
								result = memberSev < searchSev
							case EQ:
								result = memberSev == searchSev
							default :
								result = true //What happened?
						}
						//Invert logic to determine delete
						if !result {
							//Remove
							//fmt.Printf("Remove by Severity %s: %v\n", filterArray[j].SearchTerm, memberInstance)
							keepArray = keepArray[:len(keepArray)-1]
							break
						}
					} else {
						//fmt.Printf("ERROR invalid severities: %d %d\n", memberSev, searchSev)
					}
				} else {
					//All other not time options are string searches, ignore Comparator
					if !strings.Contains(currentMember.(string), filterArray[j].SearchTerm){
						//Remove
						//fmt.Printf("Remove by text %s:%s: %v\n", filterArray[j].Category, filterArray[j].SearchTerm, memberInstance)
						keepArray = keepArray[:len(keepArray)-1]
						break
					}
				}
			}
		}
	}

	//We have all logs we don't want in an array, put those in our output
	var returnArr []interface{}
	//fmt.Printf("Keep only these indexes: %v\n", keepArray)
	for _, index := range keepArray {
		returnArr = append (returnArr, membersArr[index])
	}
	return returnArr
}


func handleCollectionQueryOptions(r *http.Request, d *HTTPCmdProcessedData) *HTTPCmdProcessedData {
	// the following query parameters affect how we return collections:
	skip := r.URL.Query().Get("$skip")
	top := r.URL.Query().Get("$top")
	filter := r.URL.Query().Get("$filter")
	//fmt.Printf("Top: %s, skip:%s\n", top, skip)
	res, ok := d.Results.(map[string]interface{})
	if !ok {
		// can't be a collection if it's not a map[string]interface{} (or, rather, we can't handle it here and would need to completely re-do this with introspection.)
		//fmt.Printf("ERROR: res\n")
		return d
	}

	members, ok := res["Members"]
	if !ok {
		//fmt.Printf("ERROR: members\n")
		return d
	}

	membersArr, ok := members.([]interface{})
	if !ok {
		//fmt.Printf("ERROR: membersArr\n")
		return d
	}

	if filter != "" {
		membersArr = handleCollectionFilter(filter, membersArr)
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

	if top != "" {
		topI, err := strconv.Atoi(top)
		// TODO: http error on invalid top request
		if err == nil && topI > 0 && topI < len(membersArr) {
			// top means return exactly that many (or fewer), so slice off the end
			membersArr = membersArr[:topI]
			res["Members"] = membersArr

			// since we sliced off the end, add a nextlink user can follow to
			// get the rest (per redfish spec)
			// we'll be nice and preserve all the original query options
			q := r.URL.Query()
			q.Set("$skip", strconv.Itoa(skipI+topI))
			nextlink := url.URL{Path: r.URL.Path, RawQuery: q.Encode()}
			res["Members@odata.nextlink"] = nextlink.String()
		}
	}

	return d
}

func handleExpand(r *http.Request, d *HTTPCmdProcessedData) *HTTPCmdProcessedData {
	//expand = r.URL.Query().Get("$expand")
	return d
}

func handleSelect(r *http.Request, d *HTTPCmdProcessedData) *HTTPCmdProcessedData {
	q := r.URL.Query()
	selAry, ok := q["$select"]
	if !ok {
		return d
	}

	selectQuery := [][]string{}
	for _, j := range selAry {
		for _, i := range strings.Split(j, ",") {
			selectQuery = append(selectQuery, strings.Split(i, "/"))

			// WORKAROUND FOR BROKEN MSM
			selectQuery = append(selectQuery, strings.Split("Attributes/"+i, "/"))
			selectQuery = append(selectQuery, []string{"Id"})
			selectQuery = append(selectQuery, []string{"Name"})
			selectQuery = append(selectQuery, []string{"Description"})
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
func trimSelect(r interface{}, selAry [][]string) {
	res, ok := r.(map[string]interface{})
	if !ok {
		fmt.Printf("Could not trim no map[string]interface{} item: %s = %T\n", r, r)
		return
	}

	for k, _ := range res {
		found := false
		if strings.HasPrefix(k, "@") {
			found = true
		}
		if !found {
			for _, n := range selAry {
				if len(n[0]) == 0 {
					// let's not try to select nothing
					continue
				}
				re := regexp.MustCompile(strings.Replace(n[0], "*", ".*", -1))
				if re.MatchString(k) {
					found = true
					if len(n) <= 1 {
						break
					}

					newQuery := [][]string{}
					for _, n := range selAry {
						if k == n[0] {
							newQuery = append(newQuery, n[1:])
						}
					}
					trimSelect(res[k], newQuery)
				}
			}
		}
		if !found {
			delete(res, k)
		}
	}

	return
}
