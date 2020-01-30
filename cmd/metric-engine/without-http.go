// +build !http

package main

import (
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/log"
)

type service struct {
}

func starthttp(logger log.Logger, cfgMgr *viper.Viper, d *busComponents) *service {
	return &service{}
}

func (s *service) shutdown() {
}
