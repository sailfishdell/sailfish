package model

import ()

//  Options: these are construction-time functional options that can be passed
//  to the constructor, or after construction, you can pass them with
//  ApplyOptions

// UpdateProperty is a functional option to set an option at construction time or update the value after using ApplyOption.
// Model is locked for Options in ApplyOption
func UpdateProperty(p string, v interface{}) Option {
	return func(s *Model) error {
		s.properties[p] = v
		return nil
	}
}

// PropertyOnce is used to set a property, will fail (panic) if somebody else has already set it
func PropertyOnce(p string, v interface{}) Option {
	return func(s *Model) error {
		if _, ok := s.properties[p]; ok {
			panic("Property " + p + " can only be set once")
		}
		s.properties[p] = v
		return nil
	}
}
