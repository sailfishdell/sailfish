package model

import (
	eh "github.com/looplab/eventhorizon"
)

type propgetter interface {
	GetProperty(string) interface{}
}

func GetUUID(s propgetter) eh.UUID {
	return s.GetProperty("id").(eh.UUID)
}

func GetOdataID(s propgetter) string {
	return s.GetProperty("uri").(string)
}

type propgetterunlocked interface {
	GetPropertyUnlocked(string) interface{}
}

func GetOdataIDUnlocked(s propgetterunlocked) string {
	return s.GetPropertyUnlocked("uri").(string)
}

type propgetterokunlocked interface {
	GetPropertyOkUnlocked(string) (interface{}, bool)
}

// MustProperty is equivalent to GetProperty with the exception that it will
// panic if the property has not already been set. Use for mandatory properties.
// This is the unlocked version of this function.
func MustPropertyUnlocked(s propgetterokunlocked, name string) (ret interface{}) {
	ret, ok := s.GetPropertyOkUnlocked(name)
	if !ok {
		panic("Required property is not set: " + name)
	}
	return
}
