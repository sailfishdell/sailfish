package samplebackend

import (
    "github.com/superchalupa/go-redfish/src/redfishserver"
"strings"
)

var Config redfishserver.Config = redfishserver.Config{BackendFuncMap: funcMap, GetViewData: getViewData, MapURLToTemplate: mapURLToTemplate}

func mapURLToTemplate(url string) (templateName string, args map[string]string){
    templateName = url + "/index.json"
    templateName = strings.Replace(templateName, "//", "/", -1)
    templateName = strings.Replace(templateName, "/", "_", -1)
    if strings.HasPrefix(templateName, "_") {
        templateName = templateName[1:]
    }
    if strings.HasPrefix(templateName, "redfish_v1_") {
        templateName = templateName[len("redfish_v1_"):]
    }
    return
}

func getViewData(url string, templateName string, args map[string]string) map[string]string {
    var viewData map[string]string
    viewData = make(map[string]string)

    // some standard stuff that should be available
    viewData["root_UUID"] = "92384634-2938-2342-8820-489239905423"
    viewData["manager_UUID"] = "58893887-8974-2487-2389-841168418919"

    // root links
    var root_links map[string]string
    root_links = make(map[string]string)
    root_links[ "Systems" ] = "/redfish/v1/Systems"
    root_links[ "Chassis" ] = "/redfish/v1/Chassis"
    root_links[ "Managers" ] = "/redfish/v1/Managers"
    root_links[ "Tasks" ] = "/redfish/v1/TaskService"
    root_links[ "SessionService" ] = "/redfish/v1/SessionService"
    root_links[ "AccountService" ] = "/redfish/v1/AccountService"
    root_links[ "EventService" ] = "/redfish/v1/EventService"
    // viewData["root_links"] =  root_links

    // standard odata tags
    viewData["self_uri"] = url
    viewData["odata_self_id"] =  "\"@odata.id\": \"" + url + "\""
    viewData["redfish_std_copyright"] = "\"@Redfish.Copyright\": \"Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.\""

    return viewData
}
