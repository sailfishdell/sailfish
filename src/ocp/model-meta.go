package plugins

import ()

type MetaInt map[string]interface{}
type MetaOption func(*Service, MetaInt) error

func (s *Service) Meta(options ...MetaOption) (ret map[string]interface{}) {
	s.RLock()
	defer s.RUnlock()
	r := MetaInt{}
	r.ApplyOption(s, options...)
	return map[string]interface{}(r)
}

// ApplyOptions will run all of the provided options, you can give options that
// are for this specific service, or you can give base helper options. If you
// give an unknown option, you will get a runtime panic.
func (m MetaInt) ApplyOption(s *Service, options ...MetaOption) error {
	for _, o := range options {
		err := o(s, m)
		if err != nil {
			return err
		}
	}
	return nil
}

func PropGET(name string) MetaOption {
	return func(s *Service, m MetaInt) error {
		MustPropertyUnlocked(s, name)
		m["GET"] = map[string]interface{}{"plugin": string(s.PluginType()), "property": name}
		return nil
	}
}

func PropGETOptional(name string) MetaOption {
	return func(s *Service, m MetaInt) error {
		m["GET"] = map[string]interface{}{"plugin": string(s.PluginType()), "property": name}
		return nil
	}
}

func PropPATCH(name string) MetaOption {
	return func(s *Service, m MetaInt) error {
		MustPropertyUnlocked(s, name)
		m["PATCH"] = map[string]interface{}{"plugin": string(s.PluginType()), "property": name}
		return nil
	}
}

func PropPATCHOptional(name string) MetaOption {
	return func(s *Service, m MetaInt) error {
		m["PATCH"] = map[string]interface{}{"plugin": string(s.PluginType()), "property": name}
		return nil
	}
}
