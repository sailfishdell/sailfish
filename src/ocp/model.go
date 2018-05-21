package plugins

import (
	"sync"
)

type Option func(*Service) error

type Service struct {
	sync.RWMutex
	properties map[string]interface{}
}

func NewService(options ...Option) *Service {
	s := &Service{
		properties: map[string]interface{}{},
	}

	s.ApplyOption(options...)
	return s
}

func (s *Service) ApplyOption(options ...Option) error {
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

func (s *Service) GetProperty(p string) interface{} {
	s.RLock()
	defer s.RUnlock()
	return s.properties[p]
}
func (s *Service) GetPropertyOk(p string) (ret interface{}, ok bool) {
	s.RLock()
	defer s.RUnlock()
	ret, ok = s.properties[p]
	return
}
func (s *Service) UpdateProperty(p string, i interface{}) {
	s.Lock()
	defer s.Unlock()
	s.properties[p] = i
}

func (s *Service) GetPropertyUnlocked(p string) interface{} { return s.properties[p] }
func (s *Service) GetPropertyOkUnlocked(p string) (ret interface{}, ok bool) {
	ret, ok = s.properties[p]
	return
}
func (s *Service) UpdatePropertyUnlocked(p string, i interface{}) { s.properties[p] = i }


