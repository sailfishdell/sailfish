// Copyright (c) 2014 - Max Ekman <max@looplab.se>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package domain

import (
	"context"
	"errors"
	"fmt"

	eh "github.com/superchalupa/eventhorizon"
	"github.com/superchalupa/eventhorizon/eventhandler/projector"
)

// Invitation is a read model object for an invitation.
type Odata struct {
	ID         eh.UUID
	Properties map[string]interface{}
}

// OdataProjector is a projector that updates the Odatas.
type OdataProjector struct{}

// NewOdataProjector creates a new OdataProjector.
func NewOdataProjector() *OdataProjector {
	return &OdataProjector{}
}

// ProjectorType implements the ProjectorType method of the Projector interface.
func (p *OdataProjector) ProjectorType() projector.Type {
	return projector.Type("OdataProjector")
}

// Project implements the Project method of the Projector interface.
func (p *OdataProjector) Project(ctx context.Context, event eh.Event, model interface{}) (interface{}, error) {
	i, ok := model.(*Odata)
	if !ok {
		return nil, errors.New("model is of incorrect type")
	}

	// Apply the changes for the event.
	switch event.EventType() {
	case OdataCreatedEvent:
		data, ok := event.Data().(*OdataCreatedData)
		if !ok {
			return nil, fmt.Errorf("projector: invalid event data type: %v", event.Data())
		}
        _ = data // unused for now
        i.Properties = make(map[string]interface{})

	case OdataPropertyChangedEvent:
		data, ok := event.Data().(*OdataPropertyChangedData)
		if ok {
			i.Properties[data.PropertyName] = data.PropertyValue
		}

	default:
		return nil, errors.New("could not handle event: " + event.String())
	}

	return i, nil
}
