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

package simple

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	eh "github.com/superchalupa/eventhorizon"
	commandbus "github.com/superchalupa/eventhorizon/commandbus/local"
	eventbus "github.com/superchalupa/eventhorizon/eventbus/local"
	eventstore "github.com/superchalupa/eventhorizon/eventstore/memory"
	eventpublisher "github.com/superchalupa/eventhorizon/publisher/local"
	repo "github.com/superchalupa/eventhorizon/repo/memory"

	"github.com/superchalupa/go-redfish/src/domain"

	"testing"
)

var _ = fmt.Println
var _ = sort.Sort

func TestSimple(t *testing.T) {
	// Create the event store.
	eventStore := eventstore.NewEventStore()

	// Create the event bus that distributes events.
	eventBus := eventbus.NewEventBus()
	eventPublisher := eventpublisher.NewEventPublisher()
	eventBus.SetPublisher(eventPublisher)

	// Create the command bus.
	commandBus := commandbus.NewCommandBus()

	// Create the read repositories.
	odataRepo := repo.NewRepo()

	// Setup the domain.
	eventID := eh.NewUUID()
	domain.Setup(
		eventStore,
		eventBus,
		eventPublisher,
		commandBus,
		odataRepo,
		eventID,
	)

	// Set the namespace to use.
	ctx := eh.NewContextWithNamespace(context.Background(), "simple")

	// --- Execute commands on the domain --------------------------------------

	// IDs for all the guests.
	obj1 := eh.NewUUID()
	obj2 := eh.NewUUID()

	// Issue some invitations and responses. Error checking omitted here.
	if err := commandBus.HandleCommand(ctx, &domain.CreateOdata{OdataID: obj1, OdataURI: "/redfish/v1/happy"}); err != nil {
		log.Println("error:", err)
	}

	if err := commandBus.HandleCommand(ctx, &domain.CreateOdata{OdataID: obj2, OdataURI: "/redfish/v1/joy"}); err != nil {
		log.Println("error:", err)
	}

	time.Sleep(100 * time.Millisecond)


	if err := commandBus.HandleCommand(ctx, &domain.UpdateProperty{OdataID: obj1, PropertyName: "foo1", PropertyValue: "bar1" }); err != nil {
		log.Println("error:", err)
	}

	if err := commandBus.HandleCommand(ctx, &domain.UpdateProperty{OdataID: obj1, PropertyName: "foo2", PropertyValue: "bar2" }); err != nil {
		log.Println("error:", err)
	}

	if err := commandBus.HandleCommand(ctx, &domain.UpdateProperty{OdataID: obj1, PropertyName: "foo3", PropertyValue: "bar3" }); err != nil {
		log.Println("error:", err)
	}

	if err := commandBus.HandleCommand(ctx, &domain.UpdateProperty{OdataID: obj2, PropertyName: "foo4", PropertyValue: "bar4" }); err != nil {
		log.Println("error:", err)
	}

	if err := commandBus.HandleCommand(ctx, &domain.UpdateProperty{OdataID: obj2, PropertyName: "foo5", PropertyValue: "bar5" }); err != nil {
		log.Println("error:", err)
	}

	if err := commandBus.HandleCommand(ctx, &domain.UpdateProperty{OdataID: obj2, PropertyName: "foo6", PropertyValue: "bar6" }); err != nil {
		log.Println("error:", err)
	}

	// Wait for simulated eventual consistency before reading.
	time.Sleep(10 * time.Millisecond)

    dataObj1, err := odataRepo.Find(ctx, obj1)
    if err == nil {
		log.Println("error:", err)
    }
    dataObj2, err := odataRepo.Find(ctx, obj2)
    if err == nil {
		log.Println("error:", err)
    }
    castObj1 := dataObj1.(*domain.Odata)
    log.Printf("obj1:  foo1 == %s", castObj1.Properties["foo1"] )

    castObj2 := dataObj2.(*domain.Odata)
    log.Printf("obj2:  foo2 == %s", castObj2.Properties["foo4"] )
}
