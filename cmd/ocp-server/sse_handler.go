// Copyright (c) 2017 - Max Ekman <max@looplab.se>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	eh "github.com/looplab/eventhorizon"
)

type SSEHandler struct {
	UserName   string
	Privileges []string
	d          *DomainObjects
}

func (rh *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("SSE SERVICE\n")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// TODO: need to worry about privileges: probably should do the privilege checks in the Listener
	// to avoid races, set up our listener first
	l, err := rh.d.EventWaiter.Listen(r.Context(), func(event eh.Event) bool {
		return true
	})
	if err != nil {
		http.Error(w, "could not create waiter"+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains") // for A+ SSL Labs score
	//w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("OData-Version", "4.0")
	w.Header().Set("Server", "go-redfish")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		fmt.Printf("CLOSED SSE SESSION\n")
		l.Close()
	}()

	fmt.Fprintf(w, "TEST: 123\n\n")
	flusher.Flush()

	for {
		event, err := l.Wait(r.Context())
		fmt.Printf("GOT EVENT: %s\n", event)
		if err != nil {
			fmt.Printf("ERROR: %s\n", err.Error())
			break
		}

		data := event.Data()
		d, err := json.Marshal(data)
		fmt.Fprintf(w, "data: %s\n\n", d)

		flusher.Flush()
	}

	fmt.Printf("CLOSED\n")
}
