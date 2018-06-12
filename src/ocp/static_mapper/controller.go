package static_mapper

import (
	"context"

	"github.com/spf13/viper"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/model"
)

type staticValMapping struct {
	Property string
	Value    interface{}
}

type staticValMappingController struct {
	mappings []staticValMapping
	logger   log.Logger
	name     string
	mdl      *model.Model
}

func New(ctx context.Context, logger log.Logger, m *model.Model, name string) (*staticValMappingController, error) {
	c := &staticValMappingController{
		mappings: []staticValMapping{},
		name:     name,
		logger:   logger,
		mdl:      m,
	}

	return c, nil
}

// this is the function that viper will call whenever the configuration changes at runtime
func (c *staticValMappingController) ConfigChangedFn(ctx context.Context, cfg *viper.Viper) {

	mappings := []staticValMapping{}

	k := cfg.Sub("mappings")
	if k == nil {
		c.logger.Warn("missing config file section: 'mappings'")
		return
	}

	c.logger.Warn("unmarshaling", "name", c.name)

	err := k.UnmarshalKey(c.name, &mappings)
	if err != nil {
		c.logger.Warn("unmarshal failed", "err", err)
		return
	}

	c.createModelProperties(ctx, mappings)
}

func (c *staticValMappingController) createModelProperties(ctx context.Context, mappings []staticValMapping) {
	for _, m := range mappings {
		c.logger.Info("StaticMapper_Controller", "property", m.Property, "Value", m.Value)
		if _, ok := c.mdl.GetPropertyOkUnlocked(m.Property); !ok {
			c.logger.Info("Model property does not exist, creating: "+m.Property, "property", m.Property, "Value", m.Value)
		} else {
			// don't update if already exists
			continue
		}

		c.mdl.UpdateProperty(m.Property, m.Value)
	}
}
