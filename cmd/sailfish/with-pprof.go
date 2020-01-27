// +build pprof

package main

import (
	"net/http"
	"strings"

	// pprof automatically attaches to default servemux when imported
	_ "net/http/pprof"

	log "github.com/superchalupa/sailfish/src/log"
)

func runpprof(logger log.Logger, addShutdown func(string, interface{}), listen string) func() {
	logger.Crit("PPROF ENABLED")
	addr := strings.TrimPrefix(listen, "pprof:")
	// serve off the default http servemux that pprof attached to
	s := &http.Server{Addr: addr}
	addShutdown(listen, s)
	return func() { logger.Crit("PPROF Server exited", "err", s.ListenAndServe()) }
}
