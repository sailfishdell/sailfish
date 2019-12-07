package main

import (
	"fmt"

	"github.com/jmoiron/sqlx"

	log "github.com/superchalupa/sailfish/src/log"
)

// Factory manages getting/putting into db
type LegacyFactory struct {
	logger   log.Logger
	database *sqlx.DB
}

func NewLegacyFactory(logger log.Logger, database *sqlx.DB) (ret *LegacyFactory, err error) {
	ret = &LegacyFactory{logger: logger, database: database}
	err = nil
	return
}

func (l *LegacyFactory) Import() error {
	fmt.Printf("IMPORTING... just kidding, not really\n")
	return nil
}
