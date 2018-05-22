package attribute_resource

import (
	"fmt"

	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

type odataInt interface {
	GetProperty(string) interface{}
}

type service struct {
	*model.Service
	baseResource odataInt
}

func New(options ...interface{}) (*service, error) {
	s := &service{
		Service: model.NewService(),
	}
	s.ApplyOption(model.UUID())
	s.ApplyOption(options...)

	s.ApplyOption(model.PluginType(domain.PluginType("attribute property: " + fmt.Sprintf("%v", s.GetProperty("id")))))
	return s, nil
}

func WithUniqueName(uri string) model.Option {
	return model.PropertyOnce("unique_name", uri)
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

func WithURI(uri string) model.Option {
	return model.PropertyOnce("uri", uri)
}

func (s *service) GetURI() string {
	return s.GetProperty("uri").(string)
}
