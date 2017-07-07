package domain

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
	redfishRepo := repo.NewRepo()

	// Setup the domain.
	treeID := eh.NewUUID()
	Setup(
		eventStore,
		eventBus,
		eventPublisher,
		commandBus,
		redfishRepo,
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
	if err := commandBus.HandleCommand(ctx, &CreateRedfishResourceCollection{UUID: obj1, ResourceURI: "/", Properties: map[string]interface{}{}, Members: []string{}}); err != nil {
		log.Println("error:", err)
	}
	if err := commandBus.HandleCommand(ctx, &CreateRedfishResource{UUID: obj2, ResourceURI: "/foo", Properties: map[string]interface{}{}}); err != nil {
		log.Println("error:", err)
	}
	if err := commandBus.HandleCommand(ctx, &CreateRedfishResource{UUID: obj3, ResourceURI: "/bar", Properties: map[string]interface{}{}}); err != nil {
		log.Println("error:", err)
	}
	if err := commandBus.HandleCommand(ctx, &CreateRedfishResource{UUID: obj4, ResourceURI: "/baz", Properties: map[string]interface{}{}}); err != nil {
		log.Println("error:", err)
	}

	fmt.Println("snooze")
	if err := commandBus.HandleCommand(ctx, &AddRedfishResourceProperty{UUID: obj1, PropertyName: "snooze", PropertyValue: "42"}); err != nil {
		log.Println("error:", err)
	}

	fmt.Println("obj2_prop")
	if err := commandBus.HandleCommand(ctx, &AddRedfishResourceProperty{UUID: obj2, PropertyName: "obj2_prop", PropertyValue: "43"}); err != nil {
		log.Println("error:", err)
	}

	fmt.Println("obj3_prop")
	if err := commandBus.HandleCommand(ctx, &AddRedfishResourceProperty{UUID: obj3, PropertyName: "obj3_prop", PropertyValue: "44"}); err != nil {
		log.Println("error:", err)
	}

	rawTree, err := redfishRepo.Find(ctx, treeID)
	if err != nil {
		fmt.Printf("could not find tree: %s\n", err.Error())
	}

	tree, ok := rawTree.(*RedfishTree)
	if !ok {
		fmt.Printf("somehow it wasnt a tree! %s\n", err.Error())
	}

	fmt.Printf("/: %#v\n", tree.Tree["/"])
	rootRaw, err := redfishRepo.Find(ctx, tree.Tree["/"])
	if err != nil {
		fmt.Printf("could not find tree: %s\n", err.Error())
	}
	root, ok := rootRaw.(*RedfishResource)
	fmt.Printf("\t(%s)--> %#v\n", ok, root)

	fmt.Printf("/foo: %#v\n", tree.Tree["/foo"])
	fooRaw, err := redfishRepo.Find(ctx, tree.Tree["/foo"])
	if err != nil {
		fmt.Printf("could not find tree: %s\n", err.Error())
	}
	root, ok = fooRaw.(*RedfishResource)
	fmt.Printf("\t(%s)--> %#v\n", ok, root)

	fmt.Printf("/bar: %#v\n", tree.Tree["/bar"])
	barRaw, err := redfishRepo.Find(ctx, tree.Tree["/bar"])
	if err != nil {
		fmt.Printf("could not find tree: %s\n", err.Error())
	}
	root, ok = barRaw.(*RedfishResource)
	fmt.Printf("\t(%s)--> %#v\n", ok, root)

	fmt.Printf("/baz: %#v\n", tree.Tree["/baz"])
	bazRaw, err := redfishRepo.Find(ctx, tree.Tree["/baz"])
	if err != nil {
		fmt.Printf("could not find tree: %s\n", err.Error())
	}
	root, ok = bazRaw.(*RedfishResource)
	fmt.Printf("\t(%s)--> %#v\n", ok, root)
}
