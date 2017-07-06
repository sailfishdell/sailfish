package domain

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	eh "github.com/superchalupa/eventhorizon"
	"github.com/superchalupa/eventhorizon/eventhandler/projector"
)

var (
	ErrNotFound = errors.New("Resource could not be found")
)

type OdataResource struct {
	ID          eh.UUID
	ResourceURI string
	Properties  map[string]interface{}
}

type OdataProjector struct{}

func NewOdataProjector() *OdataProjector {
	return &OdataProjector{}
}

func (o *OdataProjector) ProjectorType() projector.Type { return projector.Type("OdataProjector") }

func (o *OdataProjector) Project(ctx context.Context, event eh.Event, model interface{}) (interface{}, error) {
	item, ok := model.(*OdataResource)
	if !ok {
		return nil, errors.New("model is the wrong type")
	}

	switch event.EventType() {
	case OdataResourceCreatedEvent:
		data, ok := event.Data().(*OdataResourceCreatedData)
		if !ok {
			return nil, errors.New("projector: invalid event data")
		}
		item.ResourceURI = data.ResourceURI
		item.Properties = map[string]interface{}{}
		for k, v := range data.Properties {
			item.Properties[k] = v
		}
	case OdataResourcePropertyAddedEvent:
		if data, ok := event.Data().(*OdataResourcePropertyAddedData); ok {
			item.Properties[data.PropertyName] = data.PropertyValue
		}
	case OdataResourcePropertyUpdatedEvent:
		if data, ok := event.Data().(*OdataResourcePropertyUpdatedData); ok {
			item.Properties[data.PropertyName] = data.PropertyValue
		}
	case OdataResourcePropertyRemovedEvent:
		if data, ok := event.Data().(*OdataResourcePropertyRemovedData); ok {
			delete(item.Properties, data.PropertyName)
		}
	case OdataResourceRemovedEvent:
		// no-op
	default:
		return nil, errors.New("Could not handle event: " + event.String())
	}

	return item, nil
}

type OdataTree struct {
	ID   eh.UUID
	Tree map[string]eh.UUID
}

func (t *OdataTree) GetOdataResourceFromTree(ctx context.Context, repo eh.ReadRepo, resourceURI string) (ret *OdataResource, err error) {
	resource, err := repo.Find(ctx, t.Tree[resourceURI])
	if err != nil {
		return nil, ErrNotFound
	}
	ret, ok := resource.(*OdataResource)
	if !ok {
		return nil, ErrNotFound
	}
	return
}

func (tree *OdataTree) WalkOdataResourceTree(ctx context.Context, repo eh.ReadRepo, start *OdataResource, path ...string) (ret *OdataResource, err error) {
	var nextP, currentP interface{}
	current := start
	currentP = current.Properties
	fmt.Printf("Walking\n")
	for _, p := range path {
		fmt.Printf("\tElement: %s\n", p)
		switch currentP := currentP.(type) {
		case map[string]interface{}:
			nextP = currentP[p]
			fmt.Printf("\t\tmap result: %s\n", nextP)
		case []interface{}:
			i, err := strconv.Atoi(p)
			if err != nil {
				return nil, errors.New("Next descent is an array, but have non-numeric index.")
			}
			nextP = currentP[i]
			fmt.Printf("\t\tarray result: %s\n", nextP)
		default:
			fmt.Printf("\t\tOh My!\n")
			return nil, errors.New("non-indexable element")
		}
		currentP = nextP

		if p == "@odata.id" {
			fmt.Printf("\t\twarp!\n")
			current, err = tree.GetOdataResourceFromTree(ctx, repo, currentP.(string))
			if err != nil {
				return nil, err
			}
			currentP = current.Properties
			continue
		}
	}

	fmt.Printf("\t\tRETURN: %#v\n", current)
	return current, nil
}

type OdataTreeProjector struct {
	repoMu sync.Mutex
	repo   eh.ReadWriteRepo
	treeID eh.UUID
}

func NewOdataTreeProjector(repo eh.ReadWriteRepo, treeID eh.UUID) *OdataTreeProjector {
	return &OdataTreeProjector{
		treeID: treeID,
		repo:   repo,
	}
}

// HandlerType implements the HandlerType method of the EventHandler interface.
func (p *OdataTreeProjector) HandlerType() eh.EventHandlerType {
	return eh.EventHandlerType("OdataTreeProjector")
}

func (t *OdataTreeProjector) HandleEvent(ctx context.Context, event eh.Event) error {
	t.repoMu.Lock()
	defer t.repoMu.Unlock()

	// load tree
	var tree *OdataTree
	m, err := t.repo.Find(ctx, t.treeID)
	if rrErr, ok := err.(eh.RepoError); ok && rrErr.Err == eh.ErrModelNotFound {
		tree = &OdataTree{ID: t.treeID, Tree: map[string]eh.UUID{}}
	} else if err != nil {
		return err
	} else {
		tree, ok = m.(*OdataTree)
		if !ok {
			return errors.New("got a model I can't handle")
		}
	}

	var _ = tree

	switch event.EventType() {
	case OdataResourceCreatedEvent:
		if data, ok := event.Data().(*OdataResourceCreatedData); ok {
			tree.Tree[data.ResourceURI] = data.UUID
		}
	case OdataResourceRemovedEvent:
		// TODO
	}

	if err := t.repo.Save(ctx, t.treeID, tree); err != nil {
		return errors.New("projector: could not save: " + err.Error())
	}

	return nil
}
