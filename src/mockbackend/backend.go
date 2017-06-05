package mockbackend

import (
	"encoding/json"
	redfishserver "github.com/superchalupa/go-redfish/src/redfishserver"
	"text/template"
    "context"
)

// Config This is the backend plugin configuration for this backend
var Config redfishserver.Config = redfishserver.Config{
    BackendFuncMap: funcMap,
    GetViewData: getViewData,
    GetJSONOutput: getJSONOutput,
}

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

var globalViewData interface{}

func init() {
	err := json.Unmarshal(initialMockupData, &globalViewData)
	if err != nil {
		panic(err)
	}
}

func getViewData(ctx context.Context, templateName, url string, args map[string]string) (viewData map[string]interface{}, err error) {
	viewData = make(map[string]interface{})
	for k, v := range globalViewData.(map[string]interface{}) {
		viewData[k] = v
	}

	// standard static tags that are useful in the templates
	viewData["self_uri"] = url
	viewData["odata_self_id"] = "\"@odata.id\": \"" + url + "\""

	return
}

func getJSONOutput(ctx context.Context, url string, args map[string]string) (output interface{}, err error) {
    switch url {
        case  "/redfish/":
            output = make(map[string]string)
            output.(map[string]string)["v1"] = "/redfish/v1/"
    }

    return
}
