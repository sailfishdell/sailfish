package attribute_resource

import (
	"fmt"

	plugins "github.com/superchalupa/go-redfish/src/ocp"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

type odataInt interface {
	GetProperty(string) interface{}
}

type service struct {
	*plugins.Service
	baseResource odataInt
}

func New(options ...interface{}) (*service, error) {
	s := &service{
		Service: plugins.NewService(),
	}
	s.ApplyOption(plugins.UUID())
	s.ApplyOption(options...)

	s.ApplyOption(plugins.PluginType(domain.PluginType("attribute property: " + fmt.Sprintf("%v", s.GetProperty("id")))))
	return s, nil
}

func WithUniqueName(uri string) plugins.Option {
	return plugins.PropertyOnce("unique_name", uri)
}

func (s *service) GetUniqueName() string {
	return s.GetProperty("unique_name").(string)
}

func BaseResource(b odataInt) Option {
	return func(p *service) error {
		p.baseResource = b
		return nil
	}
}

func WithURI(uri string) plugins.Option {
	return plugins.PropertyOnce("uri", uri)
}

func (s *service) GetURI() string {
	return s.GetProperty("uri").(string)
}
