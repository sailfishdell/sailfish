package testaggregate

import (
	"context"
	"errors"

	"github.com/Knetic/govaluate"
	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/log"
	am2 "github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/view"
)

type EventService interface {
	PublishResourceUpdatedEventsForModel(context.Context, string) view.Option
}

func RegisterWithURI(s *Service) {
	s.RegisterViewFunction("with_URI", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		exprStr, ok := cfg.(string)
		if !ok {
			logger.Crit("Failed to type assert cfg to string", "cfg", cfg)
			return errors.New("Failed to type assert expression to string")
		}
		expr, err := govaluate.NewEvaluableExpressionWithFunctions(exprStr, functions)
		if err != nil {
			logger.Crit("Failed to create evaluable expression", "expr", expr, "err", err)
			return errors.New("Failed to create evaluable expression")
		}
		uri, err := expr.Evaluate(parameters)
		if err != nil {
			logger.Crit("expression evaluation failed", "expr", expr, "err", err)
			return errors.New("expression evaluation failed")
		}
		uriStr, ok := uri.(string)
		if !ok {
			logger.Crit("expression returned non-string", "exprStr", exprStr)
			return errors.New("expression returned non-string")
		}

		logger.Debug("Registering view with URI", "expr", exprStr, "uri", uriStr)
		vw.ApplyOption(view.WithURI(uriStr))

		return nil
	})
}

func RegisterPublishEvents(s *Service, evtSvc EventService) {
	s.RegisterViewFunction("PublishResourceUpdatedEventsForModel", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		modelName, ok := cfg.(string)
		if !ok {
			logger.Crit("Failed to type assert cfg to string", "cfg", cfg)
			return errors.New("Failed to type assert expression to string")
		}

		logger.Debug("Running PublishResourceUpdatedEventsForModel", "modelName", modelName)
		vw.ApplyOption(evtSvc.PublishResourceUpdatedEventsForModel(ctx, modelName))

		return nil
	})
}

func RegisterAM2(s *Service, am2Svc *am2.Service) {
	s.RegisterControllerFunction("AM2", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
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
			logger.Crit("Required parameter 'cfgsection' missing, cannot continue")
			return errors.New("Required parameter 'cfgsection' missing, cannot continue")
		}
		cfgSectionStr, ok := cfgSection.(string)
		if !ok {
			logger.Crit("Required parameter 'cfgsection' could not be cast to string")
			return errors.New("Required parameter 'cfgsection' could not be cast to string")
		}

		uniqueName, ok := cfgParams["uniquename"]
		if !ok {
			logger.Crit("Required parameter 'uniquename' missing, cannot continue")
			return errors.New("Required parameter 'uniquename' missing, cannot continue")
		}
		uniqueNameStr, ok := uniqueName.(string)
		if !ok {
			logger.Crit("Required parameter 'uniquename' could not be cast to string")
			return errors.New("Required parameter 'uniquename' could not be cast to string")
		}

		expr, err := govaluate.NewEvaluableExpressionWithFunctions(uniqueNameStr, functions)
		if err != nil {
			logger.Crit("Failed to create evaluable expression", "uniqueNameStr", uniqueNameStr, "err", err)
			return err
		}
		uniqueName, err = expr.Evaluate(parameters)
		if err != nil {
			logger.Crit("expression evaluation failed", "expr", expr, "err", err, "cfgSection", cfgSectionStr, "uniqueName", uniqueNameStr)
			return err
		}

		uniqueNameStr, ok = uniqueName.(string)
		if err != nil {
			logger.Crit("could not cast result to string", "uniqueName", uniqueName)
			return errors.New("could not cast result to string")
		}

		logger.Debug("Creating awesome_mapper2 controller", "modelName", modelNameStr, "cfgSection", cfgSectionStr, "uniqueName", uniqueNameStr)
		am2Svc.NewMapping(ctx, logger, cfgMgr, vw.GetModel(modelNameStr), cfgSectionStr, uniqueNameStr, parameters)

		return nil
	})
}
