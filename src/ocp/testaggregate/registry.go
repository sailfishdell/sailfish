package testaggregate

import (
	"context"
	"errors"

	"github.com/Knetic/govaluate"
	"github.com/looplab/eventhorizon"
	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/log"
	am2 "github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

type EventService interface {
	PublishResourceUpdatedEventsForModel(context.Context, string) view.Option
}

type actionService interface {
	WithAction(context.Context, string, string, view.Action) view.Option
}
type pumpService interface {
	NewPumpAction(int) func(context.Context, eventhorizon.Event, *domain.HTTPCmdProcessedData) error
}

func RegisterPumpAction(s *Service, actionSvc actionService, pumpSvc pumpService) {
	s.RegisterViewFunction("with_PumpHandledAction", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		cfgParams, ok := cfg.(map[interface{}]interface{})
		if !ok {
			logger.Crit("Failed to type assert cfg to string", "cfg", cfg)
			return errors.New("Failed to type assert expression to string")
		}

		actionName, ok := cfgParams["name"]
		if !ok {
			logger.Crit("Config file missing action name for action", "cfg", cfg)
			return nil
		}
		actionNameStr, ok := actionName.(string)
		if !ok {
			logger.Crit("Action name isnt a string", "cfg", cfg)
			return nil
		}

		actionURIFrag, ok := cfgParams["uri"]
		if !ok {
			logger.Crit("Config file missing action URI for action", "cfg", cfg)
			return nil
		}
		actionURIFragStr, ok := actionURIFrag.(string)
		if !ok {
			logger.Crit("Action URI isnt a string", "cfg", cfg)
			return nil
		}

		actionTimeout, ok := cfgParams["timeout"]
		if !ok {
			logger.Crit("Config file missing action URI for action", "cfg", cfg)
			return nil
		}
		actionTimeoutInt, ok := actionTimeout.(int)
		if !ok {
			logger.Crit("Action timeout isn't a number", "cfg", cfg)
			return nil
		}

		logger.Info("Registering pump handled action", "name", actionNameStr, "URI fragment", actionURIFragStr, "timeout", actionTimeoutInt)
		vw.ApplyOption(actionSvc.WithAction(ctx, actionNameStr, actionURIFragStr, pumpSvc.NewPumpAction(actionTimeoutInt)))

		return nil
	})

	s.RegisterViewFunction("withModel", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		cfgParams, ok := cfg.(map[interface{}]interface{})
		if !ok {
			logger.Crit("Failed to type assert cfg to string", "cfg", cfg)
			return errors.New("Failed to type assert expression to string")
		}

		modelName, ok := cfgParams["name"]
		if !ok {
			logger.Crit("Config file missing model name for action", "cfg", cfg)
			return nil
		}
		modelNameStr, ok := modelName.(string)
		if !ok {
			logger.Crit("model name isnt a string", "cfg", cfg)
			return nil
		}

		modelExpr, ok := cfgParams["expr"]
		if !ok {
			logger.Crit("Config file missing model name for action", "cfg", cfg)
			return nil
		}
		modelExprStr, ok := modelExpr.(string)
		if !ok {
			logger.Crit("model name isnt a string", "cfg", cfg)
			return nil
		}
		expr, err := govaluate.NewEvaluableExpressionWithFunctions(modelExprStr, functions)
		if err != nil {
			logger.Crit("Failed to create evaluable expression", "expr", expr, "err", err)
			return errors.New("Failed to create evaluable expression")
		}
		modelVar, err := expr.Evaluate(parameters)
		if err != nil {
			logger.Crit("expression evaluation failed", "expr", expr, "err", err)
			return errors.New("expression evaluation failed")
		}
		logger.Info("WithModel", "name", modelName, "exprStr", modelExprStr)
		vw.ApplyOption(view.WithModel(modelNameStr, modelVar.(*model.Model)))

		return nil
	})

	s.RegisterViewFunction("linkModel", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		cfgParams, ok := cfg.(map[interface{}]interface{})
		if !ok {
			logger.Crit("Failed to type assert cfg to string", "cfg", cfg)
			return errors.New("Failed to type assert expression to string")
		}

		existing, ok := cfgParams["existing"]
		if !ok {
			logger.Crit("Config file missing model name for action", "cfg", cfg)
			return nil
		}
		existingStr, ok := existing.(string)
		if !ok {
			logger.Crit("model name isnt a string", "cfg", cfg)
			return nil
		}

		linkname, ok := cfgParams["linkname"]
		if !ok {
			logger.Crit("Config file missing model name for action", "cfg", cfg)
			return nil
		}
		linknameStr, ok := linkname.(string)
		if !ok {
			logger.Crit("model name isnt a string", "cfg", cfg)
			return nil
		}

		logger.Info("WithModel", "name", existingStr, "exprStr", linknameStr)
		vw.ApplyOption(view.WithModel(linknameStr, vw.GetModel(existingStr)))

		return nil
	})

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
