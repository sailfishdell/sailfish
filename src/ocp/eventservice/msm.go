package eventservice

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/go-stomp/stomp"
	"github.com/golang/protobuf/proto"
	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/ocp/eventservice/alertmodel"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

/* Temporary glocal event ID counter until MSM team decides how we can get the real ID */
var gMSMEventID int = 0

func StartEventSource(d *domain.DomainObjects, params interface{}) error {
	fmt.Printf("Starting MSM Event source\n")
	conn, err := stomp.Dial("tcp", "localhost:61613", stomp.ConnOpt.HeartBeat(time.Minute, 0))
	if err != nil {
		return err
	}
	go msmEventSource(d, conn)
	return nil
}

func msmSevToString(msmSev int32) string {
	switch msmSev {
	case 2:
		return "Informational"
	default:
		return fmt.Sprintf("Unknown %d", msmSev)
	}
}

func msmEventSource(d *domain.DomainObjects, conn *stomp.Conn) {
	sub, err := conn.Subscribe("/topic/Events", stomp.AckClient)
	if err != nil {
		log.Printf("Cannot subscribe to events! %v\n", err)
		return
	}
	for {
		for msg := range sub.C {
			if msg.Err != nil {
				log.Printf("Error getting message %v\n", msg.Err)
				continue
			}
			log.Printf("Headers: %v\nContent-Type: %s\n", msg.Header, msg.ContentType)
			contentLength, ok, err := msg.Header.ContentLength()
			log.Printf("Length is %v\nBody size is %v\n", contentLength, len(msg.Body))
			if !ok || err != nil {
				log.Printf("Message contruction error: %v\n", err)
				continue
			}
			newMessage := &alertmodel.AlertModelPayloadMessage{}
			err = proto.Unmarshal(msg.Body, newMessage)
			if err != nil {
				log.Printf("Error decoding message %v\n", err)
				continue
			}
			log.Printf("Decoded proto message %#v\n", newMessage)
			d.EventBus.PublishEvent(context.Background(),
				eh.NewEvent(RedfishEvent, &RedfishEventData{
					EventType:      "Alert",
					EventId:        fmt.Sprintf("Redfish%d", gMSMEventID),
					EventTimestamp: newMessage.TimeStamp,
					Severity:       msmSevToString(newMessage.SeverityType),
					Message:        newMessage.Message,
					MessageId:      newMessage.AlertMessageId,
					MessageArgs:    strings.Split(newMessage.MessageArgs, ","),
				}, time.Now()))
			gMSMEventID++
		}
		/*
			        <- time.After(5*time.Second)
				d.EventBus.PublishEvent(context.Background(),
			            eh.NewEvent(RedfishEvent, &RedfishEventData{
				       EventType:      "Alert",
				       EventId:        "TestID",
			               EventTimestamp: time.Now().Format("2006-01-02T15:04:05-07:00"),
			               Severity:       "Info",
			               Message:        "TestMessage",
			               MessageId:      "TestMessageID",
			               MessageArgs:    make([]string,0),
			               OriginOfCondition: "Test.Test.Test",
			        }, time.Now()))*/
	}
}
