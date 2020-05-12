// +build persistence

package main

import (
	"context"

	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/cmd/metric-engine/persistence"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
)

// nolint: gochecknoinits
// have to have init() function to runtime register the compile-time optional components, dont see any other clean way to do this
func init() {
	initOptional()
	optionalComponents = append([]func(log.Logger, *viper.Viper, busIntf) func(){
		func(logger log.Logger, cfg *viper.Viper, d busIntf) func() {
			am3JSONPersist, _ := am3.StartService(context.Background(), log.With(logger, "module", "AM3_JSON_Persist"), "Persistence", d)
			persistence.Handler(context.Background(), logger, cfg, am3JSONPersist, d)
			return nil
		}}, optionalComponents...)
}
