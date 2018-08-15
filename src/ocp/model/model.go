package model

import (
	"sync"
)

// Option is the type for functions that we use in the constructor or as args to ApplyOptions
// They are generally closure functions that mutate a model
type Option func(*Model) error

// Observer is a function that is called whenever a model is changed. It is always called with the model Locked!
type Observer func(m *Model, updates []Update)

type Update struct {
	Property string
	NewValue interface{}
}

// Model is a type that represents a bag of properties that can be formatted by a view for display
type Model struct {
	sync.RWMutex
	properties map[string]interface{}
	observers  map[string]Observer
	in_notify  int
	updates    []Update
}

// New is the constructor for a model
func New(options ...Option) *Model {
	s := &Model{
		properties: map[string]interface{}{},
		observers:  map[string]Observer{},
		updates:    []Update{},
		in_notify:  0,
	}

	s.ApplyOption(options...)
	return s
}

// UnderLock lets you run a function under the model lock.
func (s *Model) UnderLock(fn func()) {
	s.Lock()
	defer s.Unlock()
	fn()
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

func (m *Model) StopNotifications() {
	m.Lock()
	defer m.Unlock()
	m.StopNotificationsUnlocked()
}

func (m *Model) StartNotifications() {
	m.Lock()
	defer m.Unlock()
	m.StartNotificationsUnlocked()
}

func (m *Model) StopNotificationsUnlocked() {
	m.in_notify = m.in_notify + 1
}

func (m *Model) StartNotificationsUnlocked() {
	m.in_notify = m.in_notify - 1
}

func (m *Model) NotifyObservers() {
	m.Lock()
	defer m.Unlock()
	m.NotifyObserversUnlocked()
}

// caller must lock
func (m *Model) NotifyObserversUnlocked() {
	// Avoid recursive calls to observers: if an observer does a model update,
	// we don't call out to observers again. The model is locked the entire
	// time, so outside entities looking at the model will always see the
	// entire set of updates at once
	if m.in_notify <= 0 && len(m.updates) > 0 {
		m.StopNotificationsUnlocked()
		updates := m.updates
		m.updates = []Update{}
		for _, fn := range m.observers {
			fn(m, updates)
		}
		m.StartNotificationsUnlocked()
	}
}

// UpdatePropertyUnlocked is used to change properties. It will also notify any
// observers. Caller must already have locked the model
func (s *Model) UpdatePropertyUnlocked(p string, i interface{}) {
	s.properties[p] = i
	s.updates = append(s.updates, Update{p, i})
	s.NotifyObserversUnlocked()
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
