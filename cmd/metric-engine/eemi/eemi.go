package eemi

import (
	"encoding/json"
	"errors"
	"strings"
)

type Registry struct {
	msgregPath  string
	builtinMsgs map[string]map[string][4]string
}

const (
	okStr   = "OK"
	warnStr = "Warning"
	infoStr = "Informational"

	RegNameBase  = "Base"
	RegNameIDRAC = "IDRAC"
)

// For now: store these in handcoded stuff right here for initial testing
// Next patch: move these into a messages.yaml (open/read on demand, dont keep in memory)
// final (optional) form: open up the base
func NewMessageRegistry(msgregPath string) (*Registry, error) {
	return &Registry{
		msgregPath: msgregPath,
		// Registry: MesageID: {details}
		builtinMsgs: map[string]map[string][4]string{ // version, message, severity, resolution
			RegNameIDRAC: {
				"GeneralError": {"2.1", "There was a problem completing the request.", infoStr, "No response action is required."},
				"SYS413":       {"2.1", "The operation successfully completed.", infoStr, "No response action is required."},
				"SWC0283":      {"2.1", "The specified object value is not valid.", warnStr, "Enter a valid value for the corresponding object."},
			},
			RegNameBase: {
				"lookuperror": {"1.0", "could not find EEMI Message", okStr, "internal error. no response action available."},
				"Success":     {"1.5", "Successfully Completed Request", okStr, "None"},
			},
		},
	}, nil
}

const messageIDNumRequiredComponents = 2 // instantiate messages with RegistryName.MessageID

// TODO: look up messages in XML. for now adding a few here to get us started
func (l Registry) LookupMessage(messageID string) ([]string, error) {
	var err error = nil
	var regName, msgName string
	comps := strings.Split(messageID, ".")
	if len(comps) == messageIDNumRequiredComponents {
		regName = comps[0]
		msgName = comps[1]
	} else {
		regName = RegNameBase
		msgName = "lookupError"
		err = errors.New("EEMI MESSAGE LOOKUP ERROR: " + messageID)
	}

	reg, ok := l.builtinMsgs[regName]
	if !ok {
		regName = RegNameBase
		msgName = "lookuperror"
		reg = l.builtinMsgs[regName]
		err = errors.New("EEMI MESSAGE LOOKUP ERROR: " + messageID)
	}

	msg, ok := reg[msgName]
	if !ok {
		regName = RegNameBase
		msgName = "lookuperror"
		reg = l.builtinMsgs[regName]
		msg = reg[msgName]
	}

	reply := make([]string, 0, 4)
	reply = append(reply, regName+"."+msg[0]+"."+msgName)
	reply = append(reply, msg[1:]...)

	return reply, err
}

type MessageRegistry interface {
	LookupMessage(string) ([]string, error)
}

type EEMI struct {
	resolver   MessageRegistry
	wrappedErr error

	MessageID         string   `json:"MessageId"`
	MessageArgs       []string `json:"MessageArgs"`
	RelatedProperties []string `json:"RelatedProperties"`
}

func WrapErrorAsEEMI(mf MessageRegistry, err error, messageID string, args []string, relatedproperties []string) EEMI {
	return EEMI{
		wrappedErr: err,
		resolver:   mf,

		MessageID:         messageID,
		MessageArgs:       args,
		RelatedProperties: relatedproperties,
	}
}

func NewEEMI(mf MessageRegistry, messageID string, args []string, relatedproperties []string) EEMI {
	if args == nil {
		args = []string{}
	}
	if relatedproperties == nil {
		relatedproperties = []string{}
	}
	return EEMI{
		resolver:   mf,
		wrappedErr: nil,

		MessageID:         messageID,
		MessageArgs:       args,
		RelatedProperties: relatedproperties,
	}
}

func (m EEMI) Unwrap() error { return m.wrappedErr }
func (m EEMI) Error() string {
	if m.wrappedErr != nil {
		return m.wrappedErr.Error()
	}
	return "error message not found"
}

func (m EEMI) MarshalJSON() ([]byte, error) {
	type Alias EEMI
	target := struct {
		Alias
		Message    string `json:"Message"`
		Severity   string `json:"Severity"`
		Resolution string `json:"Resolution"`
	}{
		Alias: (Alias)(m),
	}
	// eat errors?
	msgdet, _ := m.resolver.LookupMessage(target.MessageID)
	target.MessageID = msgdet[0]
	target.Message = msgdet[1]
	target.Severity = msgdet[2]
	target.Resolution = msgdet[3]

	return json.Marshal(target)
}
