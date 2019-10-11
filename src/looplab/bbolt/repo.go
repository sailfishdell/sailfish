package memory

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"os"

	eh "github.com/looplab/eventhorizon"

	bbolt "github.com/etcd-io/bbolt"
)

type namespace string

// Repo implements an in memory repository of read models.
type Repo struct {
	db *bbolt.DB
}

// NewRepo creates a new Repo.
func NewRepo() *Repo {
	os.Remove("./sailfish.db")
	t, err := bbolt.Open("sailfish.db", 0600, nil)
	if err != nil {
		panic("Failed to start bbolt DB")
	}
	return &Repo{db: t}
}

// Parent implements the Parent method of the eventhorizon.ReadRepo interface.
func (r *Repo) Parent() eh.ReadRepo {
	return nil
}

// Find implements the Find method of the eventhorizon.ReadRepo interface.
func (r *Repo) Find(ctx context.Context, id eh.UUID) (ret eh.Entity, err error) {
	r.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(eh.NamespaceFromContext(ctx)))
		if b == nil {
			// bucket doesnt exist: we dont care.
			return nil
		}
		v := b.Get([]byte(id))
		if v == nil {
			// entity doesn't exist: we dont care
			return nil
		}
		decoder := gob.NewDecoder(bytes.NewReader(v))
		err := decoder.Decode(&ret)
		if err != nil {
			// couldnt decode it: academically interesting... why?
			fmt.Println("decode fail: ", err)
		}
		return nil
	})

	if ret == nil {
		return nil, eh.RepoError{
			Err:       eh.ErrEntityNotFound,
			Namespace: eh.NamespaceFromContext(ctx),
		}
	}

	return ret, nil
}

// FindAll implements the FindAll method of the eventhorizon.ReadRepo interface.
func (r *Repo) FindAll(ctx context.Context) (ret []eh.Entity, err error) {
	var model eh.Entity
	r.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(eh.NamespaceFromContext(ctx)))
		if b == nil {
			return nil
		}
		b.ForEach(func(k, v []byte) error {
			decoder := gob.NewDecoder(bytes.NewReader(v))
			err := decoder.Decode(&model)
			if err != nil {
				fmt.Println("decode fail: ", err)
				return nil
			}
			ret = append(ret, model)
			return nil
		})
		return nil
	})
	return ret, nil
}

var ErrEntityEncodeError = errors.New("error encoding entity")

// Save implements the Save method of the eventhorizon.WriteRepo interface.
func (r *Repo) Save(ctx context.Context, entity eh.Entity) error {
	if entity.EntityID() == eh.UUID("") {
		return eh.RepoError{
			Err:       eh.ErrCouldNotSaveEntity,
			BaseErr:   eh.ErrMissingEntityID,
			Namespace: eh.NamespaceFromContext(ctx),
		}
	}

	//encode entity
	encodedEntity := new(bytes.Buffer)
	encoder := gob.NewEncoder(encodedEntity)
	err := encoder.Encode(&entity)
	if err != nil {
		fmt.Println("error in encoding Entity", err)
		return eh.RepoError{
			Err:       eh.ErrCouldNotSaveEntity,
			BaseErr:   ErrEntityEncodeError,
			Namespace: eh.NamespaceFromContext(ctx),
		}
	}

	r.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(eh.NamespaceFromContext(ctx)))
		if err != nil {
			fmt.Println("Bucket Not Created: ", err)
			return nil
		}

		err = b.Put([]byte(entity.EntityID()), encodedEntity.Bytes())
		if err != nil {
			fmt.Println("Save error:", err)
		}

		return err
	})

	return nil
}

func (r *Repo) Remove(ctx context.Context, id eh.UUID) error {
	r.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(eh.NamespaceFromContext(ctx)))
		if b == nil {
			return nil
		}

		err := b.Delete([]byte(id))
		if err != nil {
			fmt.Println("Delete error", err)
		}
		return err
	})
	return nil
}

func (r *Repo) namespace(ctx context.Context) namespace {
	return namespace(eh.NamespaceFromContext(ctx))
}

// Repository returns a parent ReadRepo if there is one.
func Repository(repo eh.ReadRepo) *Repo {
	fmt.Println("Repository function called")
	if repo == nil {
		return nil
	}

	if r, ok := repo.(*Repo); ok {
		return r
	}

	return Repository(repo.Parent())
}
