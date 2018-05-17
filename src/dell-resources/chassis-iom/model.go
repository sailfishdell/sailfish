package iom_chassis

import (
    "fmt"

	plugins "github.com/superchalupa/go-redfish/src/ocp"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

type service struct {
	*plugins.Service
}

func New(options ...interface{}) (*service, error) {
	s := &service{
		Service: plugins.NewService(),
	}
	s.ApplyOption(plugins.UUID())
	s.ApplyOption(options...)
	s.ApplyOption(plugins.PropertyOnce("uri", "/redfish/v1/Chassis/"+s.GetProperty("unique_name").(string)))
	s.ApplyOption(plugins.PluginType(domain.PluginType("attribute property: " + fmt.Sprintf("%v", s.GetProperty("id")))))
	return s, nil
}

func WithUniqueName(uri string) plugins.Option {
	return plugins.PropertyOnce("unique_name", uri)
}

func (s *service) GetUniqueName() string {
	return s.GetProperty("unique_name").(string)
}
