package view

type MetaInt map[string]interface{}
type MetaOption func(*View, MetaInt) error

// TODO: this doesnt' need to be a method on *service. It could just take as parameter.
func (s *View) Meta(options ...MetaOption) (ret map[string]interface{}) {
	s.RLock()
	defer s.RUnlock()
	r := MetaInt{}
	r.ApplyOption(s, options...)
	return map[string]interface{}(r)
}

// ApplyOptions will run all of the provided options, you can give options that
// are for this specific service, or you can give base helper options. If you
// give an unknown option, you will get a runtime panic.
func (m MetaInt) ApplyOption(s *View, options ...MetaOption) error {
	for _, o := range options {
		err := o(s, m)
		if err != nil {
			return err
		}
	}
	return nil
}

func GETFormatter(formatterName string) MetaOption {
	return func(s *View, m MetaInt) error {
		getRaw, ok := m["GET"]
		if !ok {
			getRaw = map[string]interface{}{}
		}

		get, ok := getRaw.(map[string]interface{})
		if !ok {
			get = map[string]interface{}{}
		}

		get["plugin"] = string(s.PluginType())
		get["formatter"] = formatterName

		m["GET"] = get
		return nil
	}
}

func GETModel(model string) MetaOption {
	return func(s *View, m MetaInt) error {
		getRaw, ok := m["GET"]
		if !ok {
			getRaw = map[string]interface{}{}
		}

		get, ok := getRaw.(map[string]interface{})
		if !ok {
			get = map[string]interface{}{}
		}

		get["plugin"] = string(s.PluginType())
		get["model"] = model

		m["GET"] = get
		return nil
	}
}

func GETProperty(property string) MetaOption {
	return func(s *View, m MetaInt) error {
		getRaw, ok := m["GET"]
		if !ok {
			getRaw = map[string]interface{}{}
		}

		get, ok := getRaw.(map[string]interface{})
		if !ok {
			get = map[string]interface{}{}
		}

		get["plugin"] = string(s.PluginType())
		get["property"] = property

		m["GET"] = get
		return nil
	}
}

// default model, default formatter
func PropGET(name string) MetaOption {
	return func(s *View, m MetaInt) error {
		m["GET"] = map[string]interface{}{"plugin": string(s.PluginType()), "property": name}
		return nil
	}
}

func PropPATCH(name string, controller string) MetaOption {
	return func(s *View, m MetaInt) error {
		m["PATCH"] = map[string]interface{}{"plugin": string(s.PluginType()), "property": name, "controller": controller}
		return nil
	}
}
