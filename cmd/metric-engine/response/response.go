package response

import (
	"encoding/json"
)

type Response struct {
	Properties map[string]interface{}
	Headers    map[string]string
	Status     int
}

func NewResponse() *Response {
	return &Response{
		Properties: map[string]interface{}{},
		Headers:    map[string]string{},
		Status:     200,
	}
}

func (ra *Response) AddPropertyExtendedInfo(property string, message Message) *Response {
	property += "@Message.ExtendedInfo"
	p, ok := ra.Properties[property]
	if !ok {
		p = []Message{}
	}
	ma, ok := p.([]Message)
	if !ok {
		ma = []Message{}
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

// TODO: look up messages
type MessageFactory struct {
}

func NewMessageFactory() *MessageFactory {
	return &MessageFactory{}
}

type Message struct {
	MessageID         string   `json:"MessageId"`
	Message           string   `json:"Message"`
	MessageArgs       []string `json:"MessageArgs"`
	RelatedProperties []string `json:"RelatedProperties"`
	Severity          string   `json:"Severity"`
	Resolution        string   `json:"Resolution"`
}

func (mf MessageFactory) NewMessage(messageID string, args []string, relatedproperties []string) Message {
	return Message{
		MessageID:         messageID,
		Message:           "LOOKUP MESSAGE",
		MessageArgs:       args,
		RelatedProperties: relatedproperties,
		Severity:          "LOOKUP SEVERITY",
		Resolution:        "LOOKUP RESOLUTION",
	}
}
