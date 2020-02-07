package domain

import (
	"encoding/gob"
	"fmt"

	eh "github.com/looplab/eventhorizon"
)

// it's incorrect to put this here, so we'll need to move out after it's all working

func init() {

	fmt.Println("registering redfishresourceproperty with gob")
	gob.Register(&RedfishResourceAggregate{})
	gob.Register(&RedfishResourceProperty{})
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
	gob.Register(WC{})
	gob.Register(eh.UUID(""))
}
