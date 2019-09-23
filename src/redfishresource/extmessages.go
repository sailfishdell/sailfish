package domain

import (
	"context"
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

//
// property level
//

type PropertyExtendedInfoMessages struct {
	propMsgs []interface{}
}

func NewPropertyExtendedInfoMessages(msgs []interface{}) *PropertyExtendedInfoMessages {
	o := &PropertyExtendedInfoMessages{}
	o.propMsgs = make([]interface{}, len(msgs))
	copy(o.propMsgs, msgs)
	return o
}

func (p *PropertyExtendedInfoMessages) GetPropertyExtendedMessages() []interface{} {
	return p.propMsgs
}

func (o *PropertyExtendedInfoMessages) Error() string {
	return "ERROR"
}

//
// object level
//

type ObjectExtendedInfoMessages struct {
	objMsgs []interface{}
}

func NewObjectExtendedInfoMessages(msgs []interface{}) *ObjectExtendedInfoMessages {
	o := &ObjectExtendedInfoMessages{}
	o.objMsgs = make([]interface{}, len(msgs))
	copy(o.objMsgs, msgs)
	return o
}

func (o *ObjectExtendedInfoMessages) GetObjectExtendedMessages() []interface{} {
	return o.objMsgs
}

func (o *ObjectExtendedInfoMessages) Error() string {
	return "ERROR"
}

//
// object level err
//

type ObjectExtendedErrorMessages struct {
	objErrs []interface{}
}

func NewObjectExtendedErrorMessages(msgs []interface{}) *ObjectExtendedErrorMessages {
	o := &ObjectExtendedErrorMessages{}
	o.objErrs = make([]interface{}, len(msgs))
	copy(o.objErrs, msgs)
	return o
}

func (o *ObjectExtendedErrorMessages) GetObjectErrorMessages() []interface{} {
	return o.objErrs
}

func (o *ObjectExtendedErrorMessages) Error() string {
	return "ERROR"
}

//
// combined
//

type CombinedPropObjInfoError struct {
	ObjectExtendedErrorMessages
	ObjectExtendedInfoMessages
	PropertyExtendedInfoMessages
	NumSuccess int
}

func (c *CombinedPropObjInfoError) GetNumSuccess() int { return c.NumSuccess }

func (c *CombinedPropObjInfoError) Error() string { return "combined" }


type PropPatcher interface {
	PropertyPatch(context.Context, map[string]interface{},*RedfishResourceAggregate, *RedfishAuthorizationProperty, *RedfishResourceProperty, *NuEncOpts, map[string]interface{}) error
}

func AddEEMIMessage(response map[string]interface{}, a *RedfishResourceAggregate, errorType string, errs *HTTP_code) error{
	fmt.Println("HSM ENteredEEMIM", errorType, errs)
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
	if errorType == "SUCCESS"{
		fmt.println("HSM E2")
		a.StatusCode = 200
		default_msg := ExtendedInfo{}
		default_msg.GetDefaultExtendedInfo()
		addToEEMIList(response , default_msg,true)
	}else if errorType == "BADJSON"{
		fmt.println("HSM E3")
		addToEEMIList(response , bad_json, false)
	}else if errorType == "BADREQUEST"{
		fmt.println("HSM E4")
		addToEEMIList(response , bad_request, false)
	}else if errorType == "PATCHERROR"{
		any_success := errs.AnySuccess()
		if any_success >= 0 {
			a.StatusCode = 200
			default_msg := ExtendedInfo{}
			//default_msg.GetDefaultExtendedInfo()
			addToEEMIList(response , default_msg,true)
		}
			
		for _, err_msg := range errs.ErrMessage() {
			//generted extended error info msg for each err
			//de-serialize err_msg here! need to turn from string into map[string]interface{}
			msg := ExtendedInfo{}
			err := json.Unmarshal([]byte(err_msg), &msg)
			if err != nil {
				//log.MustLogger("PATCH").Crit("Error could not be unmarshalled to an EEMI message")
				return errors.New("Error updating: Could not unmarshal EEMI message")
			}
			addToEEMIList(response, msg, false)
		}
	}

	if a.StatusCode != 200 {
		a.StatusCode = 400
	}
	return nil
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

func addToEEMIList(response map[string]interface{}, eemi ExtendedInfo, isSuccess bool){
	fmt.Println("HSM test")
	var respList []interface{}
	var respIntf interface{}
	if isSuccess == false {
		fmt.Println("HSM 2 test")
		_, ok:= response["error"]
		if ok{
			
			response["error"] = map[string][]interface{}{ "@Message.ExtendedInfo": []interface{}{}}
		}
		
		fmt.Println("HSM 3 test")
		respIntf = response["error"]
		t, ok:= respIntf.(map[string]interface{})
		if ok {
		        
			fmt.Println("error")
			return
		}
		fmt.Println("HSM 4 test")
		respIntf,ok = t["@Message.ExtendedInfo"]
	} else {
		_, ok:= response["@Message.ExtendedInfo"]
		if ok{
			response["@Message.ExtendedInfo"] = []interface{}{}
		}
		respIntf= response["@Message.ExtendedInfo"]
	}

	respList, ok := respIntf.([]interface{})
	if ok {
		fmt.Println("failed to make slice of interfaces")
		return
	}
	respList = append(respList, eemi.GetExtendedInfo())
	fmt.Println("HSM respList", respList)
}






type PropGetter interface {
	PropertyGet(context.Context, *RedfishResourceAggregate, *RedfishAuthorizationProperty, *RedfishResourceProperty, map[string]interface{}) error
}
