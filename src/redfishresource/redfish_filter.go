package domain

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"regexp"
)

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

func (rh *RedfishHandler) SetupAuthorization(r *http.Request, redfishResource *RedfishResourceAggregate) *RedfishAuthorizationProperty {
	var err error
	var qm url.Values

	if r.Method != "GET" {
		auth := &RedfishAuthorizationProperty{
			UserName:   rh.UserName,
			Privileges: rh.Privileges,
			Licenses:   rh.d.GetLicenses(),
		}
		return auth
	}

	// create query map
	df := redfishResource.DefaultFilter

	if len(r.URL.Query()) == 0 && df != "" {
		qm, err = url.ParseQuery(df)
		if err != nil {
			qm = r.URL.Query()
		}
	} else {
		qm = r.URL.Query()
	}

	auth := &RedfishAuthorizationProperty{
		UserName:   rh.UserName,
		Privileges: rh.Privileges,
		Licenses:   rh.d.GetLicenses(),
		Query:      qm,
		Path:       r.URL.Path,
	}

	auth.top = 50
	auth.doTop = true
	auth.sel = []string{}

	// default to top=50 to reduce cpu
	if tstr := qm.Get("$top"); tstr != "" {
		auth.top, err = strconv.Atoi(tstr)
		auth.doTop = (err == nil)
	}

	if tstr := qm.Get("$skip"); tstr != "" {
		auth.skip, err = strconv.Atoi(tstr)
		auth.doSkip = (err == nil)
	}

	if tstr := qm.Get("$filter"); tstr != "" {
		auth.filter = qm.Get("$filter")
		auth.doFilter = true
	}

	if tstr := qm.Get("$select"); tstr != "" {
		selectSetup(auth, qm["$select"])
	}

	return auth

}

// selects are flatten and cleaned up
// if a positive select is within, a negative select, then auth.doSel = false
// will return 100% positive select or negative select
func selectSetup(auth *RedfishAuthorizationProperty, selectSl []string) {
	var negSl []string
	var totSl []string

	// flatten list of Selects to list of Select strings.
	for i := 0; i < len(selectSl); i++ {
		totSl = append(totSl, strings.Split(selectSl[i], ",")...)
	}

	// get negative selects
	for i := 0; i < len(totSl); i++ {
		if totSl[i][0] == '!' {
			totSl[i] = totSl[i][0:]
			negSl = append(negSl, totSl[i][1:])
		}
	}

	// if a positive select is under a negative select filter does not need processing
	for i := 0; i < len(negSl); i++ {
		for _, s := range totSl {
			if strings.HasPrefix(negSl[i], s) {
				auth.doSel = false
				auth.sel = []string{}
				return
			}
		}
	}

	auth.doSel = true
	if len(negSl) == len(totSl) {
		auth.selT = false
		cleanNegSelects(negSl)
		auth.sel = negSl
	} else {
		auth.selT = true
		removeNegSelects(negSl, totSl)
		auth.sel = totSl
	}
}

func cleanNegSelects(negSl []string) {
	for _, n := range negSl {
		n = n[1:]
	}
}

// Removes negative selects, because positive selects cover negative ones!
func removeNegSelects(negSl []string, selSl []string) {
	tmp := selSl[:0]
	var cnt int
	for _, n := range negSl {
		cnt = 0
		for _, s := range selSl {
			if n != s {
				tmp = append(tmp, s)
			} else {
				cnt += 1
			}
		}
		selSl = selSl[:len(selSl)-cnt]
	}
}

// TODO:  if filter has a query parameter with '$' not supported,  and extended error should be returned with the requested query parameter(s) not supported.
// RedfishFilter in runmeta would reduce the amount of loops.
func (rh *RedfishHandler) DoFilter(auth *RedfishAuthorizationProperty, data *HTTPCmdProcessedData) {
	if auth == nil {
		return
	}

	if auth.doSkip || auth.doTop || auth.doFilter {
		data = handleCollectionQueryOptions(auth, data)
	}

	if auth.doSel {
		data = handleSelect(auth, data)
	}
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
	logPathL := []string{"Oem", "Dell", c}
	mVal, ok = getValueWithPath(memberInstance, logPathL)

	return mVal, ok
}

// returns true if the memberInstance matches with the url filter, stored as filterArray
func processFilterOneObject(memberInstance map[string]interface{}, filterArray []FilterTest) bool {

	for j := 0; j < len(filterArray); j += 1 {
		currentMember, rc := getCategoryValue(memberInstance, filterArray[j].Category)
		if !rc {
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

func handleCollectionQueryOptions(a *RedfishAuthorizationProperty, d *HTTPCmdProcessedData) *HTTPCmdProcessedData {
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
			a.Query.Set("$skip", strconv.Itoa(a.skip+a.top))
			a.Query.Set("$top", strconv.Itoa(a.top))
			nextlink := url.URL{Path: a.Path, RawQuery: a.Encode}
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

func handleSelect(a *RedfishAuthorizationProperty, d *HTTPCmdProcessedData) *HTTPCmdProcessedData {
	if !a.doSel || !a.selT { // negative select is handled internally, but can be added here!
		return d
	}

	// Leaving debug prints commented out because this is hairy and they'll be needed when we revisit this
	//fmt.Printf("SELECT: %s\n", a.sel)

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
	for _, j := range a.sel {
		for _, i := range strings.Split(j, ",") {
			makesel(&selectQuery, strings.Split(i, "/"))

			// WORKAROUND FOR BROKEN MSM for Attribute selects
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

func isHeader(path string) bool {
	// something will always need @meta expanstion
	if strings.Contains(path, "@") ||
		strings.Contains(path, "Name") ||
		strings.Contains(path, "Description") ||
		strings.Contains(path, "Attributes") ||
		strings.Contains(path, "Id") ||
		strings.Contains(path, "AttributeRegistry") {
		return true
	} else {
		return false
	}

}

// designed to be used within runmeta.helper function
// arg[0] current path within recursion.
// arg[1] slice of arrays for the select filter.
// arg[2] select type true - positive, false - negative
// return is true with a slice of select strings or
// return is false with a nil slice
func selectCheck(path string, selectSl []string, sel_type bool) (bool, []string) {
	var newselect []string // == nil
	if selectSl == nil {
		return false, selectSl
	}

	for i := 0; i < len(selectSl); i++ {
		selM := selectSl[i]
		if sel_type && (strings.HasPrefix(selM, path) || strings.HasPrefix(path, selM)) { //positive select
			newselect = append(newselect, selM)
		} else if !sel_type && !strings.HasPrefix(path, selM) { // negative select
			newselect = append(newselect, selM)
		}

	}
	return len(newselect) > 0, newselect

}

//TODO: regex still matches more than it should be matching
// now we check here....
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
					//fmt.Println("FOUND", k)
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
