package simple

import (
	"context"
	"fmt"
	"log"

	eh "github.com/superchalupa/eventhorizon"
	commandbus "github.com/superchalupa/eventhorizon/commandbus/local"
	eventbus "github.com/superchalupa/eventhorizon/eventbus/local"
	eventstore "github.com/superchalupa/eventhorizon/eventstore/memory"
	eventpublisher "github.com/superchalupa/eventhorizon/publisher/local"
	repo "github.com/superchalupa/eventhorizon/repo/memory"

	"github.com/superchalupa/go-rfs/domain"
	"testing"
)

var _ = fmt.Println

func TestExample(t *testing.T) {
	fmt.Println("TESTING")

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
	treeID := eh.NewUUID()
	domain.Setup(
		eventStore,
		eventBus,
		eventPublisher,
		commandBus,
		odataRepo,
		treeID,
	)

	// Set the namespace to use.
	ctx := eh.NewContextWithNamespace(context.Background(), "simple")

	// --- Execute commands on the domain --------------------------------------

	// IDs for all the guests.
	obj1 := eh.NewUUID()
	obj2 := eh.NewUUID()
	obj3 := eh.NewUUID()
	obj4 := eh.NewUUID()

	// Issue some invitations and responses. Error checking omitted here.
	if err := commandBus.HandleCommand(ctx, &domain.CreateOdataCollection{UUID: obj1, OdataURI: "/", Properties: map[string]interface{}{}, Members: map[string]string{}}); err != nil {
		log.Println("error:", err)
	}
	if err := commandBus.HandleCommand(ctx, &domain.CreateOdata{UUID: obj2, OdataURI: "/foo", Properties: map[string]interface{}{}}); err != nil {
		log.Println("error:", err)
	}
	if err := commandBus.HandleCommand(ctx, &domain.CreateOdata{UUID: obj3, OdataURI: "/bar", Properties: map[string]interface{}{}}); err != nil {
		log.Println("error:", err)
	}
	if err := commandBus.HandleCommand(ctx, &domain.CreateOdata{UUID: obj4, OdataURI: "/baz", Properties: map[string]interface{}{}}); err != nil {
		log.Println("error:", err)
	}

	fmt.Println("snooze")
	if err := commandBus.HandleCommand(ctx, &domain.AddOdataProperty{UUID: obj1, PropertyName: "snooze", PropertyValue: "42"}); err != nil {
		log.Println("error:", err)
	}

	fmt.Println("obj2_prop")
	if err := commandBus.HandleCommand(ctx, &domain.AddOdataProperty{UUID: obj2, PropertyName: "obj2_prop", PropertyValue: "43"}); err != nil {
		log.Println("error:", err)
	}

	fmt.Println("obj3_prop")
	if err := commandBus.HandleCommand(ctx, &domain.AddOdataProperty{UUID: obj3, PropertyName: "obj3_prop", PropertyValue: "44"}); err != nil {
		log.Println("error:", err)
	}

	rawTree, err := odataRepo.Find(ctx, treeID)
	if err != nil {
		fmt.Printf("could not find tree: %s\n", err.Error())
	}

	tree, ok := rawTree.(*domain.OdataTree)
	if !ok {
		fmt.Printf("somehow it wasnt a tree! %s\n", err.Error())
	}

	fmt.Printf("/: %#v\n", tree.Tree["/"])
	rootRaw, err := odataRepo.Find(ctx, tree.Tree["/"])
	if err != nil {
		fmt.Printf("could not find tree: %s\n", err.Error())
	}
	_, ok = rootRaw.(domain.OdataAggregate)

	fmt.Printf("/foo: %#v\n", tree.Tree["/foo"])
	fooRaw, err := odataRepo.Find(ctx, tree.Tree["/foo"])
	if err != nil {
		fmt.Printf("could not find tree: %s\n", err.Error())
	}
	_, ok = fooRaw.(domain.OdataAggregate)

	fmt.Printf("/bar: %#v\n", tree.Tree["/bar"])
	barRaw, err := odataRepo.Find(ctx, tree.Tree["/bar"])
	if err != nil {
		fmt.Printf("could not find tree: %s\n", err.Error())
	}
	_, ok = barRaw.(domain.OdataAggregate)

	fmt.Printf("/baz: %#v\n", tree.Tree["/baz"])
	bazRaw, err := odataRepo.Find(ctx, tree.Tree["/baz"])
	if err != nil {
		fmt.Printf("could not find tree: %s\n", err.Error())
	}
	_, ok = bazRaw.(domain.OdataAggregate)

	/*

		// Read the guest list.
		guestList, err := guestListRepo.Find(ctx, eventID)
		if err != nil {
			log.Println("error:", err)
		}
		if l, ok := guestList.(*domain.GuestList); ok {
			log.Printf("guest list: %d invited - %d accepted, %d declined - %d confirmed, %d denied\n",
				l.NumGuests, l.NumAccepted, l.NumDeclined, l.NumConfirmed, l.NumDenied)
			fmt.Printf("guest list: %d invited - %d accepted, %d declined - %d confirmed, %d denied\n",
				l.NumGuests, l.NumAccepted, l.NumDeclined, l.NumConfirmed, l.NumDenied)
		}

		// Output:
		// invitation: Athena - confirmed
		// invitation: Hades - confirmed
		// invitation: Poseidon - denied
		// invitation: Zeus - declined
		// guest list: 4 invited - 3 accepted, 1 declined - 2 confirmed, 1 denied
	*/
}
