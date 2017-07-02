package domain

import (
	"context"
	"errors"
	"sync"

	eh "github.com/superchalupa/eventhorizon"
	"github.com/superchalupa/eventhorizon/eventhandler/projector"
)

type OdataItem struct {
	ID         eh.UUID
	OdataURI   string
	Properties map[string]interface{}
}

type OdataProjector struct{}

func NewOdataProjector() *OdataProjector {
	return &OdataProjector{}
}

func (o *OdataProjector) ProjectorType() projector.Type { return projector.Type("OdataProjector") }

func (o *OdataProjector) Project(ctx context.Context, event eh.Event, model interface{}) (interface{}, error) {
	item, ok := model.(*OdataItem)
	if !ok {
		return nil, errors.New("model is the wrong type")
	}

	switch event.EventType() {
	case OdataCreatedEvent:
		data, ok := event.Data().(*OdataCreatedData)
		if !ok {
			return nil, errors.New("projector: invalid event data")
		}
		item.OdataURI = data.OdataURI
		item.Properties = map[string]interface{}{}
		for k, v := range data.Properties {
			item.Properties[k] = v
		}
	case OdataPropertyAddedEvent:
		if data, ok := event.Data().(*OdataPropertyAddedData); ok {
			item.Properties[data.PropertyName] = data.PropertyValue
		}
	case OdataPropertyUpdatedEvent:
		if data, ok := event.Data().(*OdataPropertyUpdatedData); ok {
			item.Properties[data.PropertyName] = data.PropertyValue
		}
	case OdataPropertyRemovedEvent:
		if data, ok := event.Data().(*OdataPropertyRemovedData); ok {
			delete(item.Properties, data.PropertyName)
		}
	case OdataRemovedEvent:
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
	case OdataCreatedEvent:
		if data, ok := event.Data().(*OdataCreatedData); ok {
			tree.Tree[data.OdataURI] = data.UUID
		}
	case OdataRemovedEvent:
		// TODO
	}

	if err := t.repo.Save(ctx, t.treeID, tree); err != nil {
		return errors.New("projector: could not save: " + err.Error())
	}

	return nil
}
