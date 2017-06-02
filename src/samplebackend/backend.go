package samplebackend

import (
	"fmt"
	"github.com/superchalupa/go-redfish/src/redfishserver"
	"net/http"
	"regexp"
	"strings"
)

var Config redfishserver.Config = redfishserver.Config{BackendFuncMap: funcMap, GetViewData: getViewData, MapURLToTemplate: mapURLToTemplate}

func mapURLToTemplate(r *http.Request) (templateName string, args map[string]string, err error) {
	url := r.URL.Path
	args = make(map[string]string)

	templateName = url + "/index.json"
	templateName = strings.Replace(templateName, "//", "/", -1)
	templateName = strings.Replace(templateName, "/", "_", -1)
	if strings.HasPrefix(templateName, "_") {
		templateName = templateName[1:]
	}
	if strings.HasPrefix(templateName, "redfish_v1_") {
		templateName = templateName[len("redfish_v1_"):]
	}

	var systemRegexp = regexp.MustCompile("^/redfish/v1/Systems/([a-zA-Z0-9]+)")
	if system := systemRegexp.FindSubmatch([]byte(r.URL.Path)); system != nil {
		fmt.Printf("Found a system URL: %s\n", system[1])
		templateName = "System_template.json"
		args["System"] = string(system[1])
	}

	return
}

type odataLink struct {
	Name   string
	Target string
}

var globalViewData map[string]interface{}

func init() {
	// allocate
	globalViewData = make(map[string]interface{})

	// root links
	var root_links []odataLink
	root_links = append(root_links, odataLink{Name: "Systems", Target: "/redfish/v1/Systems"})
	root_links = append(root_links, odataLink{Name: "Chassis", Target: "/redfish/v1/Chassis"})
	root_links = append(root_links, odataLink{Name: "Tasks", Target: "/redfish/v1/TaskService"})
	root_links = append(root_links, odataLink{Name: "SessionService", Target: "/redfish/v1/SessionService"})
	root_links = append(root_links, odataLink{Name: "AccountService", Target: "/redfish/v1/AccountService"})
	root_links = append(root_links, odataLink{Name: "EventService", Target: "/redfish/v1/EventService"})
	globalViewData["root_links"] = root_links

	// some standard stuff that should be available
	globalViewData["root_UUID"] = "92384634-2938-2342-8820-489239905423"
	globalViewData["manager_UUID"] = "58893887-8974-2487-2389-841168418919"

	// copyright (static)
	globalViewData["redfish_std_copyright"] = "\"@Redfish.Copyright\": \"Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.\""

	// in order to properly output JSON without trailing commas, need the list of systems in an array (maps have trailing comma issue)
	var systemList []string
	globalViewData["systemList"] = systemList

	// Then we make a sub-map to hold the actual data for each system
	var systems map[string]interface{}
	systems = make(map[string]interface{})
	// now hook the systems into viewdata
	globalViewData["systems"] = systems

	// System 437XR1138R2
	var system_437XR1138R2 map[string]string
	system_437XR1138R2 = make(map[string]string)
	systems["437XR1138R2"] = system_437XR1138R2
	systemList = append(systemList, "437XR1138R2")
	system_437XR1138R2["name"] = "WebFrontEnd483"
	system_437XR1138R2["SystemType"] = "Physical"
	system_437XR1138R2["AssetTag"] = "Chicago-45Z-2381"
	system_437XR1138R2["Manufacturer"] = "Contoso"
	system_437XR1138R2["Model"] = "3500RX"
	system_437XR1138R2["SKU"] = "8675309"
	system_437XR1138R2["SerialNumber"] = "437XR1138R2"
	system_437XR1138R2["PartNumber"] = "224071-J23"
	system_437XR1138R2["Description"] = "Web Front End node"
	system_437XR1138R2["UUID"] = "38947555-7742-3448-3784-823347823834"
	system_437XR1138R2["HostName"] = "web483"

	// System "dummy"
	var system_dummy map[string]string
	system_dummy = make(map[string]string)
	systems["dummy"] = system_dummy
	systemList = append(systemList, "dummy")
	system_dummy["name"] = "a dummy system"
}

func getViewData(r *http.Request, templateName string, args map[string]string) (viewData map[string]interface{}) {
	url := r.URL.Path

	viewData = make(map[string]interface{})
	for k, v := range globalViewData {
		viewData[k] = v
	}

	// standard static tags that are useful in the templates
	viewData["self_uri"] = url
	viewData["odata_self_id"] = "\"@odata.id\": \"" + url + "\""

	return viewData
}
