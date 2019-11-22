package view

import (
	"strconv"

	"github.com/superchalupa/sailfish/src/ocp/model"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func WithDeferRegister() Option {
	return func(s *View) error {
		s.registerplugin = false
		return nil
	}
}

func WithURI(uri string) Option {
	return func(s *View) error {
		s.pluginType = domain.PluginType(uri)
		s.viewURI = uri
		return nil
	}
}

func WithModel(name string, m *model.Model) Option {
	return func(s *View) error {
		s.models[name] = m
		return nil
	}
}

func WithFormatter(name string, g formatter) Option {
	return func(s *View) error {
		s.outputFormatters[name] = g
		return nil
	}
}

func WithController(name string, c controller) Option {
	return func(s *View) error {
		s.controllers[name] = c
		s.closefn = append(s.closefn, c.Close)
		return nil
	}
}

func WithAction(name string, a Action) Option {
	return func(s *View) error {
		s.actions[name] = a
		return nil
	}
}

func WithUpload(name string, u Upload) Option {
	return func(s *View) error {
		s.uploads[name] = u
		return nil
	}
}

func WatchModel(name string, fn func(*View, *model.Model, []model.Update)) Option {
	return func(s *View) error {
		if m, ok := s.models[name]; ok {
			m.AddObserver(s.GetURIUnlocked(), func(m *model.Model, updates []model.Update) {
				fn(s, m, updates)
			})
		}
		return nil
	}
}

func UpdateEtag(modelName string, includedProps []string) Option {
	etag := 1
	return WatchModel(modelName, func(v *View, m *model.Model, updates []model.Update) {
		// TODO: scan updates to see if it's one of the includedProps
		//      For now, do the simple things.
		etag++
		m.UpdatePropertyUnlocked("etag", `W/"genid-`+strconv.Itoa(etag)+`"`)
	})
}

func AtClose(fn func()) Option {
	return func(s *View) error {
		s.closefn = append(s.closefn, fn)
		return nil
	}
}
