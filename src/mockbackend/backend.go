package mockbackend

import (
	"encoding/json"
	"fmt"
	"github.com/superchalupa/go-redfish/src/redfishserver"
	"net/http"
	"regexp"
	"strings"
	"text/template"
)

// Config This is the backend plugin configuration for this backend
var Config redfishserver.Config = redfishserver.Config{BackendFuncMap: funcMap, GetViewData: getViewData, MapURLToTemplate: mapURLToTemplate}

var funcMap = template.FuncMap{
	"hello": func() string { return "HELLO WORLD" },
}

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

var initialMockupData = []byte(`{
    "root_links": [
        {"name": "Systems", "target": "/redfish/v1/Systems" },
        {"name": "Chassis", "target": "/redfish/v1/Chassis" },
        {"name": "Tasks", "target": "/redfish/v1/TaskService" },
        {"name": "SessionService", "target": "/redfish/v1/SessionService" },
        {"name": "AccountService", "target": "/redfish/v1/AccountService" },
        {"name": "EventService", "target": "/redfish/v1/EventService" }
    ],
    "root_UUID": "92384634-2938-2342-8820-489239905423",
    "manager_UUID": "58893887-8974-2487-2389-841168418919",
    "redfish_std_copyright": "@Redfish.Copyright: Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.",
    "systemList": [ "437XR1138R2", "dummy"],

    "systems": {
        "437XR1138R2": {
            "name": "WebFrontEnd483",
            "SystemType": "Physical",
            "AssetTag": "Chicago-45Z-2381",
            "Manufacturer": "Contoso",
            "Model": "3500RX",
            "SKU": "8675309",
            "SerialNumber": "437XR1138R2",
            "PartNumber": "224071-J23",
            "Description": "Web Front End node",
            "UUID": "38947555-7742-3448-3784-823347823834",
            "HostName": "web483"
        },
        "dummy": {
            "name": "a dummy system"
        }
    }
}`)

var globalViewData interface{}

func init() {
	err := json.Unmarshal(initialMockupData, &globalViewData)
	if err != nil {
		fmt.Println("error:", err)
		panic(err)
	}
}

func getViewData(r *http.Request, templateName string, args map[string]string) (viewData map[string]interface{}) {
	url := r.URL.Path

	viewData = make(map[string]interface{})
	for k, v := range globalViewData.(map[string]interface{}) {
		viewData[k] = v
	}

	// standard static tags that are useful in the templates
	viewData["self_uri"] = url
	viewData["odata_self_id"] = "\"@odata.id\": \"" + url + "\""

	return viewData
}
