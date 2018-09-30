package testaggregate

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
)

/*
views:
  "testview":
      "Logger": ["module": "test_view"]
      "models":
        - "default":  {"property1": "value1"}
      "controllers":
      "view":
        - "with_URI": "/redfish/v1/testview#[foo]"
        - "with_foo": ""
        - "with_bar": ""
        - "with_aggregate": "name"
*/

type config struct {
	Logger []interface{}
}

func InstantiateFromCfg(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, name string, parameters map[string]interface{}) (*view.View, error) {
	subCfg := cfgMgr.Sub("views")
	if subCfg == nil {
		logger.Warn("missing config file section: 'views'")
		return nil, errors.New("invalid config section 'views'")
	}

	config := config{}

	err := subCfg.UnmarshalKey(name, &config)
	if err != nil {
		logger.Warn("unamrshal failed", "err", err)
	}

	fmt.Printf("GOT CONFIG: %#v\n", config)

	// Instantiate logger
	subLogger := logger.New(config.Logger...)
	subLogger.Debug("Instantiated new logger")

	// Instantiate Models

	// Instantiate controllers

	// Instantiate view
	vw := view.New()

	// Instantiate aggregate

	return vw, nil
}
