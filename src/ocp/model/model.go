package model

import (
	"sync"
)

// Option is the type for functions that we use in the constructor or as args to ApplyOptions
// They are generally closure functions that mutate a model
type Option func(*Model) error

// Observer is a function that is called whenever a model is changed. It is always called with the model Locked!
type Observer func(m *Model, property string, oldValue, newValue interface{})

// Model is a type that represents a bag of properties that can be formatted by a view for display
type Model struct {
	sync.RWMutex
	properties map[string]interface{}
	observers  map[string]Observer
	in_notify  bool
}

// New is the constructor for a model
func New(options ...Option) *Model {
	s := &Model{
		properties: map[string]interface{}{},
		observers:  map[string]Observer{},
		in_notify:  false,
	}

	s.ApplyOption(options...)
	return s
}

// ApplyOption is run with all of the options given by the constructor, but can
// also be used after construction to apply options
func (s *Model) ApplyOption(options ...Option) error {
	s.Lock()
	defer s.Unlock()
	for _, o := range options {
		err := o(s)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Model) AddObserver(name string, ob Observer) {
	s.Lock()
	defer s.Unlock()
	s.observers[name] = ob
}

// UpdatePropertyUnlocked is used to change properties. It will also notify any
// observers. Caller must already have locked the model
func (s *Model) UpdatePropertyUnlocked(p string, i interface{}) {
	old, _ := s.properties[p]
	s.properties[p] = i
	if !s.in_notify {
		s.in_notify = true
		for _, fn := range s.observers {
			fn(s, p, old, i)
		}
	}
	s.in_notify = false
}

// UpdateProperty is used to change properties. It will also notify any
// observers.
func (s *Model) UpdateProperty(p string, i interface{}) {
	s.Lock()
	defer s.Unlock()
	s.UpdatePropertyUnlocked(p, i)
}

func (s *Model) GetPropertyUnlocked(p string) interface{} { return s.properties[p] }
func (s *Model) GetPropertyOkUnlocked(p string) (ret interface{}, ok bool) {
	ret, ok = s.properties[p]
	return
}

func (s *Model) GetProperty(p string) interface{} {
	s.RLock()
	defer s.RUnlock()
	return s.properties[p]
}
func (s *Model) GetPropertyOk(p string) (ret interface{}, ok bool) {
	s.RLock()
	defer s.RUnlock()
	ret, ok = s.properties[p]
	return
}
