package fans

import (
	"github.com/superchalupa/go-redfish/plugins"
)

type Option func(*service) error

func (c *service) ApplyOption(options ...interface{}) error {
	for _, o := range options {
		var err error
		switch o := o.(type) {
		case Option:
			err = o(c)
		case plugins.Option:
			err = o(c.Service)
		}

		if err != nil {
			return err
		}
	}
	return nil
}
