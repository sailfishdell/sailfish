package samplebackend

import (
	"github.com/superchalupa/go-redfish/src/redfishserver"
	"strings"
    "net/http"
    "regexp"
    "fmt"
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

func getViewData(r* http.Request, templateName string, args map[string]string) (viewData map[string]interface{}) {
    url := r.URL.Path

	viewData = make(map[string]interface{})

	// some standard stuff that should be available
	viewData["root_UUID"] = "92384634-2938-2342-8820-489239905423"
	viewData["manager_UUID"] = "58893887-8974-2487-2389-841168418919"

	// root links
	var root_links map[string]string
	root_links = make(map[string]string)
	root_links["Systems"] = "/redfish/v1/Systems"
    // uncomment these as we get the templates in
	//root_links["Chassis"] = "/redfish/v1/Chassis"
	//root_links["Managers"] = "/redfish/v1/Managers"
	//root_links["Tasks"] = "/redfish/v1/TaskService"
	//root_links["SessionService"] = "/redfish/v1/SessionService"
	//root_links["AccountService"] = "/redfish/v1/AccountService"
	//root_links["EventService"] = "/redfish/v1/EventService"
	viewData["root_links"] = root_links

    var systems map[string]interface{}
    systems = make(map[string]interface{})

    // System 437XR1138R2
    var system_437XR1138R2 map[string]string
    system_437XR1138R2 = make( map[string]string )

    systems["437XR1138R2"] = system_437XR1138R2

    // System "dummy"
    var system_dummy map[string]string
    system_dummy = make( map[string]string )

    systems["dummy"] = system_dummy


    viewData["systems"] = systems

	// standard static tags that are useful in the templates
	viewData["self_uri"] = url
	viewData["odata_self_id"] = "\"@odata.id\": \"" + url + "\""
	viewData["redfish_std_copyright"] = "\"@Redfish.Copyright\": \"Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.\""

	return viewData
}
