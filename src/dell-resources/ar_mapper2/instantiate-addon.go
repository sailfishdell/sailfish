package ar_mapper2

import (
	"context"
	"errors"
	"sync"

	"github.com/Knetic/govaluate"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
)

func RegisterARMapper(s *testaggregate.Service, arsvc *ARService) {
	s.RegisterControllerFunction("ARMapper", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		cfgParams, ok := cfg.(map[interface{}]interface{})
		if !ok {
			logger.Crit("Failed to type assert cfg to string", "cfg", cfg)
			return errors.New("Failed to type assert expression to string")
		}

		// ctx, logger, viper, *model, cfg_name, params
		modelName, ok := cfgParams["modelname"]
		if !ok {
			modelName = "default"
		}
		modelNameStr, ok := modelName.(string)
		if !ok {
			modelNameStr = "default"
		}

		cfgSection, ok := cfgParams["cfgsection"]
		if !ok {
			logger.Crit("Required parameter 'cfgsection' missing, cannot continue", "modelname", modelNameStr, "cfg", cfg)
			return nil
		}
		cfgSectionStr, ok := cfgSection.(string)
		if !ok {
			logger.Crit("Required parameter 'cfgsection' could not be cast to string", "modelname", modelNameStr)
			return nil
		}

		mappingUniqueName, ok := cfgParams["mappinguniquename"]
		if !ok {
			logger.Crit("Required parameter 'mappinguniquename' missing, cannot continue", "modelname", modelNameStr)
			return nil
		}
		mappingUniqueNameStr, ok := mappingUniqueName.(string)
		if !ok {
			logger.Crit("Required parameter 'mappinguniquename' could not be cast to string", "modelname", modelNameStr)
			return nil
		}

		functions := awesome_mapper2.InitFunctions()
		expr, err := govaluate.NewEvaluableExpressionWithFunctions(mappingUniqueNameStr, functions)
		if err != nil {
			logger.Crit("Failed to create evaluable expression", "expr", expr, "err", err)
			return errors.New("Failed to create evaluable expression")
		}
		mappingName, err := expr.Evaluate(parameters)
		if err != nil {
			logger.Crit("expression evaluation failed", "expr", expr, "err", err)
			return errors.New("expression evaluation failed")
		}
		mappingNameStr, ok := mappingName.(string)
		if !ok {
			logger.Crit("expression returned non-string", "mappingUniqueName", mappingUniqueName)
			return errors.New("expression returned non-string")
		}

		addToView, ok := cfgParams["AddToView"]
		if !ok {
			addToView = ""
		}
		addToViewStr, ok := addToView.(string)
		if !ok {
			addToViewStr = ""
		}

		// if this stuff not present, no big deal
		passthruParams, ok := cfgParams["passthru"]
		if ok {
			testaggregate.GetPassThruParams(logger, parameters, passthruParams)
		}

		// convert params
		newparams := map[string]string{}
		for k, v := range parameters {
			vStr, ok := v.(string)
			if ok {
				newparams[k] = vStr
			}
		}

		logger.Debug("Creating ar_mapper2 controller", "modelName", modelNameStr, "cfgSection", cfgSectionStr, "mappingNameStr", mappingNameStr)
		b := arsvc.NewMapping(logger, mappingNameStr, cfgSectionStr, vw.GetModel(modelNameStr), newparams)

		if addToViewStr != "" {
			vw.ApplyOption(view.WithController(addToViewStr, b))
		}

		return nil
	})
}
