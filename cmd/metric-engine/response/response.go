package response

import (
	"encoding/json"

	"github.com/superchalupa/sailfish/cmd/metric-engine/eemi"
)

const HTTPStatusOK = 200

type Response struct {
	Properties map[string]interface{}
	Headers    map[string]string
	Status     int
}

func NewResponse() *Response {
	return &Response{
		Properties: map[string]interface{}{},
		Headers:    map[string]string{},
		Status:     HTTPStatusOK,
	}
}

func (ra *Response) AddPropertyExtendedInfo(property string, message eemi.EEMI) *Response {
	property += "@Message.ExtendedInfo"
	p, ok := ra.Properties[property]
	if !ok {
		p = []eemi.EEMI{}
	}
	ma, ok := p.([]eemi.EEMI)
	if !ok {
		ma = []eemi.EEMI{}
	}
	ma = append(ma, message)
	ra.Properties[property] = ma
	return ra
}

func (ra *Response) SetStatus(status int) *Response {
	ra.Status = status
	return ra
}

func (ra Response) GetStatus() int {
	return ra.Status
}

func (ra *Response) SetHeader(h, v string) *Response {
	ra.Headers[h] = v
	return ra
}

func (ra Response) GetHeaders() map[string]string {
	return ra.Headers
}

func (ra Response) MarshalJSON() ([]byte, error) {
	return json.Marshal(ra.Properties)
}
