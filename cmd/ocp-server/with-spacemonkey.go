// +build spacemonkey

package main

import (
    "log"
    "net/http"

	"github.com/spacemonkeygo/openssl"
)

func run_spacemonkey(addr string, handler http.HandlerFunc) {
    log.Println("OPENSSL(spacemonkey) listener starting")
    log.Fatal(openssl.ListenAndServeTLS(addr, "server.crt", "server.key", handler))
}
