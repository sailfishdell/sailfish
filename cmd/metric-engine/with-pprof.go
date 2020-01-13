// +build pprof
// +build http

package main

import (
	"net/http"
	"strings"

	// pprof automatically attaches to default servemux when imported
	_ "net/http/pprof"

	log "github.com/superchalupa/sailfish/src/log"
)

func runpprof(logger log.Logger, listen string) (func(), *http.Server) {
	logger.Crit("PPROF ENABLED")
	// the _ import of pprof will attach to the default global http servemux
	// so just serve off that one
	addr := strings.TrimPrefix(listen, "pprof:")
	s := &http.Server{Addr: addr}
	return func() { logger.Crit("PPROF Server exited", "err", s.ListenAndServe()) }, s
}
