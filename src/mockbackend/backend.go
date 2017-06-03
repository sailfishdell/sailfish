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
