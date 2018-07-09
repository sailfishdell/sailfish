package awesome_mapper

import (
    "context"
    "errors"

	"github.com/spf13/viper"
    "github.com/Knetic/govaluate"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/event"
	"github.com/superchalupa/go-redfish/src/ocp/model"
)

type mapping struct {
	Property   string
	Query string
}

type MappingEntry struct {
    Select      string
    ModelUpdate []mapping
}

func New(ctx context.Context, logger log.Logger, cfg *viper.Viper, m *model.Model, name string, parameters map[string]interface{}) (error) {
	c := []MappingEntry{}

	k := cfg.Sub("mappings")
	if k == nil {
		logger.Warn("missing config file section: 'mappings'")
		return errors.New("Missing config section 'mappings'")
	}
	err := k.UnmarshalKey(name, &c)
	if err != nil {
		logger.Warn("unmarshal failed", "err", err)
	}
	logger.Warn("updated mappings", "mappings", c)

    try := c[0]

	// stream processor for action events
	sp, err := event.NewESP(ctx, event.ExpressionFilter(logger, try.Select, parameters, map[string]govaluate.ExpressionFunction{}))
	if err != nil {
		logger.Error("Failed to create event stream processor", "err", err, "select-string", try.Select)
		return err
	}
	sp.RunForever(func(event eh.Event) {
        logger.Crit("GOT AN EVENT", "event", event)
	})

	return nil
}
