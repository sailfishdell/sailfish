package domain

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

func (e *ExtendedInfo) GetDefaultExtendedInfo() map[string]interface{}{
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

func (e *ExtendedInfo) GetExtendedInfo() map[string]interface{}{
  info := map[string]interface{}{
    "Message": e.Message,
    "MessageArgs": e.MessageArgs,
    "MessageArgs@odata.count": e.MessageArgsCt,
    "MessageId": e.MessageId,
    "RelatedProperties": e.RelatedProperties,
    "RelatedProperties@odata.count": e.RelatedPropertiesCt,
    "Resolution": e.Resolution,
    "Severity": e.Severity,
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
}

func (c *CombinedPropObjInfoError) Error() string { return "combined" }
