package mb2

import (
	"encoding/json"
	redfishserver "github.com/superchalupa/go-redfish/src/rfs2"
	"regexp"
	"strings"
	"text/template"
    "context"
)

// Config This is the backend plugin configuration for this backend
var Config redfishserver.Config = redfishserver.Config{BackendFuncMap: funcMap, GetViewData: getViewData, MapURLToTemplate: mapURLToTemplate}

/**************************************************************************
// Everything from here below is private to this module. The only interface
// from the outside world into this module is the Config above
**************************************************************************/

// This is how we add new functions to the text/template parser, so we can do
// some (minimally) more complicated processing directly inside the template
// rather than inside code
var funcMap = template.FuncMap{
	"hello": func() string { return "HELLO WORLD" },
}

func mapURLToTemplate(url string) (templateName string, args map[string]string, err error) {
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
	if system := systemRegexp.FindSubmatch([]byte(url)); system != nil {
		templateName = "System_template.json"
		args["System"] = string(system[1])
	}

	return
}

var globalViewData interface{}

func init() {
	err := json.Unmarshal(initialMockupData, &globalViewData)
	if err != nil {
		panic(err)
	}
}

func getViewData(ctx context.Context, url string, templateName string, args map[string]string) (viewData map[string]interface{}) {
	viewData = make(map[string]interface{})
	for k, v := range globalViewData.(map[string]interface{}) {
		viewData[k] = v
	}

	// standard static tags that are useful in the templates
	viewData["self_uri"] = url
	viewData["odata_self_id"] = "\"@odata.id\": \"" + url + "\""

	return viewData
}
