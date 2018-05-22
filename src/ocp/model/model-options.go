package model

import (
	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

//
//  Options: these are construction-time functional options that can be passed
//  to the constructor, or after construction, you can pass them with
//  ApplyOptions
//

// UpdateProperty is a functional option to set an option at construction time or update the value after using ApplyOption.
// Service is locked for Options in ApplyOption
func UpdateProperty(p string, v interface{}) Option {
	return func(s *Service) error {
		s.properties[p] = v
		return nil
	}
}

// Service is locked for Options in ApplyOption
func PropertyOnce(p string, v interface{}) Option {
	return func(s *Service) error {
		if _, ok := s.properties[p]; ok {
			panic("Property " + p + " can only be set once")
		}
		s.properties[p] = v
		return nil
	}
}

func URI(uri string) Option {
	return UpdateProperty("uri", uri)
}

func UUID() Option {
	return UpdateProperty("id", eh.NewUUID())
}

func PluginType(pt domain.PluginType) Option {
	return UpdateProperty("plugin_type", pt)
}
