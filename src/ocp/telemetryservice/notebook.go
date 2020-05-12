package telemetryservice

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	//"sync"
	"errors"
	"golang.org/x/xerrors"
	"sort"

	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

// internal functions
// apply wild cards to property and return list of expanded properties
func (ts *NoteBook) expandProps(metricID string, prop string, mapWC map[string][]string) []string {
	propT := []string{}
	if !strings.Contains(prop, "{") {
		propT = append(propT, prop)
		return propT
	}

	for key, vals := range mapWC {
		for i := 0; i < len(vals); i++ {
			if vals[i] == "*" {
				MM, error := ts.getMDMetricData(metricID)
				if error != nil {
					fmt.Println(error)
				}
				MD_WC := MM.wcValues
				for j := 0; j < len(MM.wcValues); j++ {
					if MD_WC[j] == "*" {
						MD_WC[j] = MD_WC[len(MM.wcValues)-1]
						MD_WC = MD_WC[len(MM.wcValues)-2:]
					}
				}
				mapWC[key] = MD_WC
				break
			}
		}
	}

	for key, vals := range mapWC {
		if strings.Contains(prop, key) {
			for i := 0; i < len(vals); i++ {
				propT = append(propT,
					strings.Replace(prop, key, vals[i], 1))
			}
		}
	}
	return propT
}

// designed to be used in one goroutine within TelemetryService
type NoteBook struct {
	// { mrdid: MRDmeta{}}
	mrds map[string]MRDmeta

	// used to check if UpdatedRedfishresource event has metrics
	// {URL: {path to prop: []*MRDmeta}
	metric2MRD map[string]map[string][]*MRDmeta // metricprop: MRD_ID
	ctx        context.Context
	d          *domain.DomainObjects
}

type MDmeta struct {
	metricId string
	wcName   string
	wcValues []string
	prop     string
}

func initTelemetryNotebook(ctx context.Context, d *domain.DomainObjects) *NoteBook {
	telemetryConfig := NoteBook{
		mrds:       map[string]MRDmeta{},
		metric2MRD: map[string]map[string][]*MRDmeta{},
		ctx:        ctx,
		d:          d,
	}

	return &telemetryConfig
}

type MRDmeta struct {
	mrdEnabled bool
	UUID       eh.UUID
	metrics    map[string][]string // {metric id:[] md prop}
	wildcard   map[string][]string // { WC Name: []WC Values}
}

// find a better place for me..
func getRedfishAggregate(ctx context.Context, d *domain.DomainObjects, id eh.UUID) (*domain.RedfishResourceAggregate, error) {
	agg, err := d.AggregateStore.Load(ctx, domain.AggregateType, id)
	if err != nil {
		return nil, errors.New("could not load subscription aggregate")
	}
	redfishResource, ok := agg.(*domain.RedfishResourceAggregate)
	if !ok {
		return nil, errors.New("wrong aggregate type returned")
	}
	return redfishResource, nil
}

// get contents within brackets
var rgx = regexp.MustCompile(`\{(.*?)\}`)

// checks if there are any MRD metrics that match with the RRPU2 data.
func (ts *NoteBook) getValidMetrics(data *domain.RedfishResourcePropertiesUpdatedData2) map[string]map[string]interface{} {
	propMatchMap := map[string]map[string]interface{}{}
	propsinMRD, ok := ts.metric2MRD[data.ResourceURI]
	if !ok {
		return nil
	}
	//fmt.Println(data.ResourceURI, "metric is found")

	// check if the props are added.
	for propPath, val := range data.PropertyNames {
		MRDs, ok := propsinMRD[propPath]
		if !ok {
			continue
		}
		//fmt.Println("prop check", propPath, propsinMRD)

		for i := 0; i < len(MRDs); i++ {
			if MRDs[i].mrdEnabled {
				// find metric id
				mID := ""

				for metricid, m := range MRDs[i].metrics {
					for j := 0; j < len(m); j++ {
						wc := rgx.FindString(m[j])
						slice := strings.Split(m[j], wc)
						prop := data.ResourceURI + "#" + propPath
						if strings.Contains(prop, slice[0]) && strings.Contains(prop, slice[1]) {
							mID = metricid
							break
						}
					}
				}
				//fmt.Println("FOUND METRIC ID", mID)

				_, ok := propMatchMap[mID]
				if ok {
					propMatchMap[mID][propPath] = val
				} else {
					propMatchMap[mID] = map[string]interface{}{propPath: val}
				}
			}
		}
	}
	return propMatchMap
}

// When MRD POST arrives.  Clean and Validate MRD contents with existing MDs
// rc : false - MRD has no good metrics
// rc : true  - MRD has at least one good metric/metricproperty
func (ts *NoteBook) CleanAndValidateMRD(data *MRDData) (bool, string) {
	// validate metric ids
	idx := 0 // output index
	errmsg := ""
	for i, _ := range data.Metrics {
		MRDmetric := data.Metrics[i]
		// see if I get the MD data correctly
		if MRDmetric.MetricID == "" {
			errmsg += "a metric id is empty, "
			continue
		}
		MM, err := ts.getMDMetricData(MRDmetric.MetricID)
		if err != nil {
			errmsg += "a metric is not present in MD, "
			// metric is not present in MD, skip adding
			continue
		} else {
			// removes bad metric data
			errmsg += cleanMRDMetric(MM, &MRDmetric)
			if len(MRDmetric.MetricProperties) == 0 {
				// metric properties is empty. skip adding
				continue
			} else {
				// good metric data
				data.Metrics[idx] = MRDmetric
				idx++
			}

		}
	}

	data.Metrics = data.Metrics[:idx]

	if idx == 0 {
		return false, errmsg
	} else {
		return true, errmsg
	}

}

// MRD wildcard validation is not done for now.
// rc: false - Metric does not have a good property
// 	true - Metric has at least one good property
func cleanMRDMetric(MM MDmeta, MRDmetric *Metric) string {
	reString := MM.prop
	substr := strings.Join(MM.wcValues, `\b|`) + `\b`
	strings.Replace(reString, MM.wcName, substr, -1)
	re := regexp.MustCompile(reString)
	errmsg := ""

	idx := 0
	for i := 0; i < len(MRDmetric.MetricProperties); i++ {
		MRDprop := MRDmetric.MetricProperties[i]
		if MRDprop == MM.prop {
			MRDmetric.MetricProperties[idx] = MM.prop
			idx++
		} else if re.MatchString(MRDprop) {
			MRDmetric.MetricProperties[idx] = MM.prop
			idx++
		} else {
			errmsg += "property " + MRDprop + " is not valid, "
			continue
		}
	}
	MRDmetric.MetricProperties = MRDmetric.MetricProperties[:idx]
	return errmsg

}

func (ts *NoteBook) getMDMetricData(metricID string) (MDmeta, error) {
	MM := MDmeta{}
	MM.metricId = metricID
	MD := "/redfish/v1/TelemetryService/MetricDefinitions/"
	mduuid, ok := ts.d.GetAggregateIDOK(MD + metricID)
	if !ok {
		return MM, errors.New("Can not get UUID for " + MD + metricID)
	}

	rra, err := getRedfishAggregate(ts.ctx, ts.d, mduuid)
	if err != nil {
		fmt.Println("MD AGG not found", mduuid)
		return MM, xerrors.Errorf("MD Aggregate not found %s", mduuid)
	}

	loc, ok := rra.Properties.Value.(map[string]interface{})
	if !ok {
		fmt.Println("MD Properties.Value is not map[string]interface{}")
		return MM, errors.New("Wildcards are not in []interface{} format")
	}

	wcIntf := domain.Flatten(loc["Wildcards"], false)
	wcIntf2, ok := wcIntf.([]interface{})
	if !ok {
		fmt.Printf("Wildcards are not in []interface{} they are in %TEND\n", wcIntf)
		return MM, errors.New("Wildcards are not in []interface{} format")
	}
	if len(wcIntf2) > 1 {
		return MM, xerrors.Errorf("MD has more than one wildcard")
	}
	wcMap, ok := wcIntf2[0].(map[string]interface{})
	if !ok {
		fmt.Printf("Wildcard is not in map[string]interfface but in %T\n", wcIntf2[0])
		return MM, errors.New("wildcard not in map format")
	}

	key, ok1 := wcMap["Name"].(string)
	values, ok2 := wcMap["Values"].([]string)
	if ok1 && ok2 {
		MM.wcName = key
		MM.wcValues = values
	}

	props := domain.Flatten(loc["MetricProperties"], false).([]interface{})
	if len(props) > 1 {
		return MM, errors.New("MD has more than one MetricProperties")
	}
	propIntf := props[0]
	MM.prop = propIntf.(string)

	return MM, nil

}

// add MRD data to NoteBook.mrds and .prop2Metric
func (ts *NoteBook) MRDConfigAdd(data *MRDData) {
	mapWC := map[string][]string{}
	for i := 0; i < len(data.Wildcards); i++ {
		key := data.Wildcards[i].Name
		values := data.Wildcards[i].Values
		_, ok := mapWC[key]
		if !ok {
			mapWC[key] = values
		} else {
			mapWC[key] = append(mapWC[key], values...)
		}
	}

	// delete WC dups
	// might move this to cleanup and validation
	for _, vals := range mapWC {
		sort.Strings(vals)
		idx := 0
		lastVal := vals[0]
		for i := 1; i < len(vals); i++ {
			// ultimate wild card
			if vals[i] == "*" {
				vals[0] = "*"
				idx = 1
				break
			}

			if vals[i] == lastVal {
				// skip
			} else {
				lastVal = vals[i]
				idx++
			}
		}
		vals = vals[:idx]
	}

	// metric ids and metricproperties are validated when MRD is first created
	// so keeping Props and metric id for reference when MRD is patched.

	metricMap := map[string][]string{}
	for _, metric := range data.Metrics {
		metricMap[metric.MetricID] = metric.MetricProperties
	}

	//fmt.Println("metricMap", metricMap)

	MM := MRDmeta{
		UUID:       data.UUID,
		mrdEnabled: data.MetricReportDefinitionEnabled,
		wildcard:   mapWC,
		metrics:    metricMap,
	}
	ts.mrds[data.Id] = MM

	// update metricstoProp used for incoming aggregate changes.
	for _, props := range metricMap {
		ts.add2metric2MRD(data.Id, props, mapWC, &MM)
	}
}

// delete MRD data to NoteBook.mrds and .prop2Metric
func (ts *NoteBook) MRDConfigDelete(URI string) {
	mrdid := strings.TrimLeft(URI, "/redfish/v1/TelemetryService/MetricReportDefinitions/")

	mrd, ok := ts.mrds[mrdid]
	if !ok {
		fmt.Println("ERROR: ", URI, "is not recognized in TelemetryService Config", mrdid, ts.mrds)
	}

	propT := []string{}
	// get all props in mrd.
	for _, props := range ts.mrds[mrdid].metrics {
		for i := 0; i < len(props); i++ {
			propT = append(propT, ts.expandProps(mrdid, props[i], ts.mrds[mrdid].wildcard)...)
		}
	}

	// cleanup metric2MRD
	// delete mrd props in metric2MRD
	for i := 0; i < len(propT); i++ {
		strSplit := strings.Split(propT[i], "#")
		url := strSplit[0]
		prop := strSplit[1]
		propMRDs := ts.metric2MRD[url][prop]
		// for wild card characters need to expand when inputting inside...
		if len(propMRDs) == 1 && propMRDs[0].UUID == mrd.UUID {
			if len(ts.metric2MRD[url]) == 1 {
				delete(ts.metric2MRD, url)
			} else {
				delete(ts.metric2MRD[url], prop)
			}
			continue
		}

		for j := 0; j < len(propMRDs); j++ {
			if propMRDs[j].UUID == mrd.UUID {
				ml := len(propMRDs) - 1
				ts.metric2MRD[url][prop][j] = ts.metric2MRD[url][prop][ml]
				ts.metric2MRD[url][prop] = ts.metric2MRD[url][prop][:ml-1]
				break
			}
		}
	}
	delete(ts.mrds, mrdid)
}

// For MRD PATCH requests..
// TODO
func (ts *NoteBook) MRDConfigUpdate(URI string) {
	// update items - metrics, wildcards -- schema says no to wildcards this is likely incorrect
	// update Enabled val as well.
	//here MRD is updated...
	// use UpdateRedfishResourceudpate2... to update agg.
}

// list of props are expanded with wildcards, and added to the TelemetryConfigDB metric2MRD
func (ts *NoteBook) add2metric2MRD(metricid string, props []string, wc map[string][]string, mrdMeta *MRDmeta) {
	for i := 0; i < len(props); i++ {
		// expand wildcards now simplify
		propT := ts.expandProps(metricid, props[i], wc)
		for j := 0; j < len(propT); j++ {
			//fmt.Println(propT[j])
			if strings.Contains(propT[j], "#") {
				split := strings.Split(propT[j], "#")
				url := split[0]
				prop := split[1]
				_, ok := ts.metric2MRD[url]
				if !ok {
					ts.metric2MRD[url] = map[string][]*MRDmeta{prop: []*MRDmeta{}}
				}
				_, ok = ts.metric2MRD[url][prop]
				if !ok {
					ts.metric2MRD[url][prop] = []*MRDmeta{}
				}
				//fmt.Println("adding new prop to metric 2MRD", url, prop, mrdMeta)
				ts.metric2MRD[url][prop] = append(ts.metric2MRD[url][prop], mrdMeta)

			}
		}
	}
}
