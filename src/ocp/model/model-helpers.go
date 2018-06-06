package model

import ()

type propgetterokunlocked interface {
	GetPropertyOkUnlocked(string) (interface{}, bool)
}

// MustPropertyUnlocked is equivalent to GetProperty with the exception that it will
// panic if the property has not already been set. Use for mandatory properties.
// This is the unlocked version of this function.
func MustPropertyUnlocked(s propgetterokunlocked, name string) (ret interface{}) {
	ret, ok := s.GetPropertyOkUnlocked(name)
	if !ok {
		panic("Required property is not set: " + name)
	}
	return
}
