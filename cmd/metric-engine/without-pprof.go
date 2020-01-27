// +build !pprof
// +build http

package main

import (
	"fmt"
	"net/http"

	log "github.com/superchalupa/sailfish/src/log"
)

func runpprof(logger log.Logger, addr string) (func(), *http.Server) {
	return func() {
		fmt.Println("PPROF IS REQUESTED, BUT NOT ENABLED WITH BUILD TAG! Add '-tag \"pprof\"' on the 'go build' command line to enable.")
	}, nil
}
