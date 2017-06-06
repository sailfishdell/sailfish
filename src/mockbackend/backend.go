package mockbackend

import (
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
            return collapseCollection(SystemCollectionJSON.(map[string]interface{}))

        case  "/redfish/v1/Systems/{system}":
            return getCollectionMember(SystemCollectionJSON.(map[string]interface{}), url)

        default:
            err = redfishserver.ErrNotFound
    }

    return
}

// collapse the "Members": [ {...}, {...} ] so that only @odata.id appears in the output array
func collapseCollection(inputJSON map[string]interface{}) (outputJSON interface{}, err error) {
    var output map[string]interface{}
    output = make(map[string]interface{})

    // range over input, copying to output
    for k,v := range inputJSON {
        // if input is "Members", filter it
        if k=="Members" {
            // make new members array
            var members []map[string]interface{}
            for _, val := range v.([]interface{}) {
                // pull out @odata.id from input and paste it into the output
                id := val.(map[string]interface{})["@odata.id"]
                members = append(members,  map[string]interface{}{"@odata.id": id})
            }
            output["Members"] = members
        } else {
            output[k] = v
        }
    }

    outputJSON = &output
    return
}

// collapse the "Members": [ {...}, {...} ] so that only @odata.id appears in the output array
func getCollectionMember(inputJSON map[string]interface{}, filter string) (interface{}, error) {
    // range over input, copying to output
    members := inputJSON["Members"]
    for _,v := range members.([]interface{}) {
        id := v.(map[string]interface{})["@odata.id"].(string)
        if id == filter {
            return v, nil
        }
    }

    return nil, redfishserver.ErrNotFound
}
