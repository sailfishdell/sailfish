package domain

//
// property level
//

type PropertyExtendedInfoMessages struct {
	propMsgs []interface{}
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

func (o *ObjectExtendedErrorMessages) GetObjectExtendedMessages() []interface{} {
	return o.objErrs
}

func (o *ObjectExtendedErrorMessages) Error() string {
	return "ERROR"
}

//
// combined
//

type CombinedInfoError struct {
	*ObjectExtendedErrorMessages
	*ObjectExtendedInfoMessages
}

func (c *CombinedInfoError) Error() string { return "combined" }
