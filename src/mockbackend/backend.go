package mockbackend

import (
    "errors"
	"encoding/json"
	redfishserver "github.com/superchalupa/go-redfish/src/redfishserver"
	"text/template"
    "context"
)

type backendConfig struct {
    templatesDir string
}

func NewBackend( templatesDir string ) redfishserver.Config {
    return  redfishserver.Config {
        GetJSONOutput: getJSONOutput,
        BackendUserdata: backendConfig{templatesDir: templatesDir},
    }
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

var (
    serviceV1RootJSON,
    SystemCollectionJSON interface{}
    )

func init() {
    var unmarshalJSONPairs = [] struct { global *interface{}; jsonText []byte }{
        {global: &serviceV1RootJSON, jsonText: serviceV1RootText},
        {global: &SystemCollectionJSON, jsonText: SystemCollectionText},
    }
    for i := range unmarshalJSONPairs {
        err := json.Unmarshal(unmarshalJSONPairs[i].jsonText, unmarshalJSONPairs[i].global)
        if err != nil {
            panic(err)
        }
    }
}

func getJSONOutput(ctx context.Context, logger redfishserver.Logger, pathTemplate, url string, args map[string]string) (output interface{}, err error) {
    switch pathTemplate {
        case  "/redfish/":
            output = make(map[string]string)
            output.(map[string]string)["v1"] = "/redfish/v1/"

        case  "/redfish/v1/":
            return &serviceV1RootJSON, nil

        case  "/redfish/v1/Systems":
            return &SystemCollectionJSON, nil

        case  "/redfish/v1/Systems/{system}":
            return &SystemCollectionJSON, nil

        default:
            err = errors.New("no handler for URL: " + url)
    }

    return
}

