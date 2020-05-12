package domain

import (
	"encoding/json"
	"errors"
	"fmt"
)

//
// extended info struct
//
type ExtendedInfo struct {
	Message             string
	MessageArgs         []string
	MessageArgsCt       int `json:"MessageArgs@odata.count"`
	MessageId           string
	RelatedProperties   []string
	RelatedPropertiesCt int `json:"RelatedProperties@odata.count"`
	Resolution          string
	Severity            string
}

func (e *ExtendedInfo) GetDefaultExtendedInfo() map[string]interface{} {
	e.Message = "Successfully Completed Request"
	e.MessageArgs = []string{}
	e.MessageArgsCt = len(e.MessageArgs)
	e.MessageId = "Base.1.0.Success"
	e.RelatedProperties = []string{}
	e.RelatedPropertiesCt = len(e.RelatedProperties)
	e.Resolution = "None"
	e.Severity = "OK"
	return e.GetExtendedInfo()
}

func (e *ExtendedInfo) GetExtendedInfo() map[string]interface{} {
	info := map[string]interface{}{
		"Message":                       e.Message,
		"MessageArgs":                   e.MessageArgs,
		"MessageArgs@odata.count":       e.MessageArgsCt,
		"MessageId":                     e.MessageId,
		"RelatedProperties":             e.RelatedProperties,
		"RelatedProperties@odata.count": e.RelatedPropertiesCt,
		"Resolution":                    e.Resolution,
		"Severity":                      e.Severity,
	}
	return info
}

type HTTP_code struct {
	Err_message []string
	Any_success int
}

func (e HTTP_code) ErrMessage() []string {
	return e.Err_message
}

func (e HTTP_code) AnySuccess() int {
	return e.Any_success
}

func (e HTTP_code) Error() string {
	return fmt.Sprintf("Request Error Message: %s", e.Err_message)
}

func AddEEMIMessage(response map[string]interface{}, a *RedfishResourceAggregate, errorType string, errs *HTTP_code) error {
	bad_json := ExtendedInfo{
		Message:             "The request body submitted was malformed JSON and could not be parsed by the receiving service.",
		MessageArgs:         []string{}, //FIX ME
		MessageArgsCt:       0,          //FIX ME
		MessageId:           "Base.1.0.MalformedJSON",
		RelatedProperties:   []string{}, //FIX ME
		RelatedPropertiesCt: 0,          //FIX ME
		Resolution:          "Ensure that the request body is valid JSON and resubmit the request.",
		Severity:            "Critical",
	}

	bad_request := ExtendedInfo{
		Message:             "The service detected a malformed request body that it was unable to interpret.",
		MessageArgs:         []string{},
		MessageArgsCt:       0,
		MessageId:           "Base.1.0.UnrecognizedRequestBody",
		RelatedProperties:   []string{"Attributes"}, //FIX ME
		RelatedPropertiesCt: 1,                      //FIX ME
		Resolution:          "Correct the request body and resubmit the request if it failed.",
		Severity:            "Warning",
	}

	if errorType == "SUCCESS" {
		a.StatusCode = 200
		default_msg := ExtendedInfo{}
		AddToEEMIList(response, default_msg, true)
	} else if errorType == "BADJSON" {
		a.StatusCode = 400
		AddToEEMIList(response, bad_json, false)
	} else if errorType == "BADREQUEST" {
		a.StatusCode = 400
		AddToEEMIList(response, bad_request, false)
	} else if errorType == "PATCHERROR" {
		any_success := errs.AnySuccess()
		if any_success > 0 {
			a.StatusCode = 200
			default_msg := ExtendedInfo{}
			AddToEEMIList(response, default_msg, true)
		} else {
			a.StatusCode = 400
		}

		for _, err_msg := range errs.ErrMessage() {
			msg := ExtendedInfo{}
			err := json.Unmarshal([]byte(err_msg), &msg)
			if err != nil {
				return errors.New("Error updating: Could not unmarshal EEMI message")
			}
			AddToEEMIList(response, msg, false)
		}
	}

	if a.StatusCode != 200 {
		a.StatusCode = 400
	}
	return nil
}

func AddToEEMIList(response map[string]interface{}, eemi ExtendedInfo, isSuccess bool) {
	extendedInfoL := &[]map[string]interface{}{}
	var ok bool

	if isSuccess {
		response["@Message.ExtendedInfo"] = extendedInfoL
		*extendedInfoL = append(*extendedInfoL, eemi.GetDefaultExtendedInfo())
		return
	}

	// not success message
	response["code"] = "Base.1.0.GeneralError"
	response["message"] = "A general error has occurred.  See ExtendedInfo for more information"

	t, ok := response["error"]
	if !ok {
		response["error"] = map[string]*[]map[string]interface{}{
			"@Message.ExtendedInfo": extendedInfoL}
		*extendedInfoL = append(*extendedInfoL, eemi.GetExtendedInfo())
	} else {

		if !ok {
			return
		}
		t2, ok := t.(map[string]*[]map[string]interface{})

		if !ok {
			return
		}
		t3 := t2["@Message.ExtendedInfo"]
		*t3 = append(*t3, eemi.GetExtendedInfo())

	}

}
