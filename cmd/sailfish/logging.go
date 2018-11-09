package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	mylog "github.com/superchalupa/sailfish/src/log"

	eh "github.com/looplab/eventhorizon"
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

// Create a tiny logging middleware for the command handler.
func makeLoggingCmdHandler(l mylog.Logger, originalHandler eh.CommandHandler) eh.CommandHandler {
	return eh.CommandHandlerFunc(func(ctx context.Context, cmd eh.Command) error {
		l.Debug("Executed Command", "Type", cmd.CommandType(), "CMD", fmt.Sprintf("%v", cmd))
		return originalHandler.HandleCommand(ctx, cmd)
	})
}
