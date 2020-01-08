package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	mylog "github.com/superchalupa/sailfish/src/log"
)

func makeLoggingHTTPHandler(l mylog.Logger, m http.Handler) http.HandlerFunc {
	// Simple HTTP request logging.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func(begin time.Time) {
			l.Info(
				"Processed http request",
				"source", r.RemoteAddr,
				"method", r.Method,
				"url", r.URL,
				"business_logic_time", time.Since(begin),
				"module", "http",
				"args", fmt.Sprintf("%#v", mux.Vars(r)),
			)
		}(time.Now())
		m.ServeHTTP(w, r)
	})
}
