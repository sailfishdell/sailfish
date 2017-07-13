package domain

import (
	"context"
	"errors"
	"strconv"
	"sync"

	eh "github.com/superchalupa/eventhorizon"
	"github.com/superchalupa/eventhorizon/eventhandler/projector"
)

var (
	ErrNotFound = errors.New("Resource could not be found")
)

type RedfishResource struct {
	ID           eh.UUID
	ResourceURI  string
	Properties   map[string]interface{}
	PrivilegeMap map[string]interface{}
	Permissions  map[string]interface{}
	Headers      map[string]string
}

type RedfishProjector struct{}

func NewRedfishProjector() *RedfishProjector {
	return &RedfishProjector{}
}

func (o *RedfishProjector) ProjectorType() projector.Type { return projector.Type("RedfishProjector") }

func (o *RedfishProjector) Project(ctx context.Context, event eh.Event, model interface{}) (interface{}, error) {
	item, ok := model.(*RedfishResource)
	if !ok {
		return nil, errors.New("model is the wrong type")
	}

	switch event.EventType() {
	case RedfishResourceCreatedEvent:
		data, ok := event.Data().(*RedfishResourceCreatedData)
		if !ok {
			return nil, errors.New("projector: invalid event data")
		}
		item.ResourceURI = data.ResourceURI
		item.Properties = map[string]interface{}{}
		for k, v := range data.Properties {
			item.Properties[k] = v
		}
		item.PrivilegeMap = map[string]interface{}{}
		item.Permissions = map[string]interface{}{}
		item.Headers = map[string]string{}
	case RedfishResourcePropertiesUpdatedEvent:
		if data, ok := event.Data().(*RedfishResourcePropertiesUpdatedData); ok {
			for k, v := range data.Properties {
				item.Properties[k] = v
			}
		}
	case RedfishResourcePropertyRemovedEvent:
		if data, ok := event.Data().(*RedfishResourcePropertyRemovedData); ok {
			delete(item.Properties, data.PropertyName)
		}
	case RedfishResourceRemovedEvent:
		// TODO ?
	case RedfishResourcePrivilegesUpdatedEvent:
		if data, ok := event.Data().(*RedfishResourcePrivilegesUpdatedData); ok {
			item.PrivilegeMap = data.Privileges
		}
	case RedfishResourcePermissionsUpdatedEvent:
		if data, ok := event.Data().(*RedfishResourcePermissionsUpdatedData); ok {
			item.Permissions = data.Permissions
		}
	case RedfishResourceHeadersUpdatedEvent:
		if data, ok := event.Data().(*RedfishResourceHeadersUpdatedData); ok {
			for k, v := range data.Headers {
				item.Headers[k] = v
			}
		}
	case RedfishResourceHeaderRemovedEvent:
		if data, ok := event.Data().(*RedfishResourceHeaderRemovedData); ok {
			delete(item.Headers, data.HeaderName)
		}
	default:
		return nil, errors.New("Could not handle event: " + event.String())
	}

	return item, nil
}

type RedfishTree struct {
	ID   eh.UUID
	Tree map[string]eh.UUID
}

// helper
func GetTree(ctx context.Context, repo eh.ReadRepo, treeID eh.UUID) (t *RedfishTree, err error) {
	rawTree, err := repo.Find(ctx, treeID)
	if err != nil {
		return nil, errors.New("could not find tree with ID: " + string(treeID) + " error is: " + err.Error())
	}

	t, ok := rawTree.(*RedfishTree)
	if !ok {
		return nil, errors.New("Data structure inconsistency, the tree object wasnt a tree!: " + string(treeID) + " error is: " + err.Error())
	}

	return
}

func (t *RedfishTree) GetRedfishResourceFromTree(ctx context.Context, repo eh.ReadRepo, resourceURI string) (ret *RedfishResource, err error) {
	resource, err := repo.Find(ctx, t.Tree[resourceURI])
	if err != nil {
		return nil, ErrNotFound
	}
	ret, ok := resource.(*RedfishResource)
	if !ok {
		return nil, ErrNotFound
	}
	return
}

func (tree *RedfishTree) WalkRedfishResourceTree(ctx context.Context, repo eh.ReadRepo, start *RedfishResource, path ...string) (ret *RedfishResource, err error) {
	var nextP, currentP interface{}
	current := start
	currentP = current.Properties
	for _, p := range path {
		switch currentP := currentP.(type) {
		case map[string]interface{}:
			nextP = currentP[p]
		case []interface{}:
			i, err := strconv.Atoi(p)
			if err != nil {
				return nil, errors.New("Next descent is an array, but have non-numeric index.")
			}
			nextP = currentP[i]
		default:
			return nil, errors.New("non-indexable element")
		}
		currentP = nextP

		if p == "@odata.id" {
			current, err = tree.GetRedfishResourceFromTree(ctx, repo, currentP.(string))
			if err != nil {
				return nil, err
			}
			currentP = current.Properties
			continue
		}
	}

	return current, nil
}

type RedfishTreeProjector struct {
	repoMu sync.Mutex
	repo   eh.ReadWriteRepo
	treeID eh.UUID
}

func NewRedfishTreeProjector(repo eh.ReadWriteRepo, treeID eh.UUID) *RedfishTreeProjector {
	return &RedfishTreeProjector{
		treeID: treeID,
		repo:   repo,
	}
}

// HandlerType implements the HandlerType method of the EventHandler interface.
func (p *RedfishTreeProjector) HandlerType() eh.EventHandlerType {
	return eh.EventHandlerType("RedfishTreeProjector")
}

func (t *RedfishTreeProjector) HandleEvent(ctx context.Context, event eh.Event) error {
	t.repoMu.Lock()
	defer t.repoMu.Unlock()

	// load tree
	var tree *RedfishTree
	m, err := t.repo.Find(ctx, t.treeID)
	if rrErr, ok := err.(eh.RepoError); ok && rrErr.Err == eh.ErrModelNotFound {
		tree = &RedfishTree{ID: t.treeID, Tree: map[string]eh.UUID{}}
	} else if err != nil {
		return err
	} else {
		tree, ok = m.(*RedfishTree)
		if !ok {
			return errors.New("got a model I can't handle")
		}
	}

	var _ = tree

	switch event.EventType() {
	case RedfishResourceCreatedEvent:
		if data, ok := event.Data().(*RedfishResourceCreatedData); ok {
			tree.Tree[data.ResourceURI] = event.AggregateID()
		}
	case RedfishResourceRemovedEvent:
		for k, v := range tree.Tree {
			if v == event.AggregateID() {
				delete(tree.Tree, k)
				// there can be only one
				break
			}
		}
	}

	if err := t.repo.Save(ctx, t.treeID, tree); err != nil {
		return errors.New("projector: could not save: " + err.Error())
	}

	return nil
}
