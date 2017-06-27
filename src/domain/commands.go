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
	eh "github.com/superchalupa/eventhorizon"
)

func init() {
	eh.RegisterCommand(func() eh.Command { return &CreateOdata{} })
	eh.RegisterCommand(func() eh.Command { return &UpdateProperty{} })
}

const (
	CreateOdataCommand    eh.CommandType = "CreateOdata"
	UpdatePropertyCommand eh.CommandType = "UpdateProperty"
)

// CreateOdata is a command for creating invites.
type CreateOdata struct {
	OdataID eh.UUID
	OdataURI  string
}

func (c CreateOdata) AggregateID() eh.UUID            { return c.OdataID }
func (c CreateOdata) AggregateType() eh.AggregateType { return OdataAggregateType }
func (c CreateOdata) CommandType() eh.CommandType     { return CreateOdataCommand }

// UpdateProperty is a command for accepting Odatas.
type UpdateProperty struct {
	OdataID       eh.UUID
	PropertyName  string
	PropertyValue interface{}
}

func (c UpdateProperty) AggregateID() eh.UUID            { return c.OdataID }
func (c UpdateProperty) AggregateType() eh.AggregateType { return OdataAggregateType }
func (c UpdateProperty) CommandType() eh.CommandType     { return UpdatePropertyCommand }
