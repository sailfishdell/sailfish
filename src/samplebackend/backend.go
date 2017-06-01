package samplebackend

import (
    "github.com/superchalupa/go-redfish/src/redfishserver"
)

var Config redfishserver.Config = redfishserver.Config{funcMap, getViewData}

func getViewData(url string) map[string]string {
    var viewData map[string]string
    viewData = make(map[string]string)

    viewData["test"] = "foo"

    return viewData
}
