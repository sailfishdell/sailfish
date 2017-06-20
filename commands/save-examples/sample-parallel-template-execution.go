package main

import (
	"text/template"
	"bytes"
	"context"
	"fmt"
	"github.com/google/uuid"
	"time"
)

type resultpair struct {
	key   string
	value string
}

func getFuncMap(ctx context.Context, resultsQueue chan resultpair) (*int, template.FuncMap) {
	totalJobs := 0
	var funcMap = template.FuncMap{
		"sleep": func() string {
			totalJobs = totalJobs + 1
			token := uuid.New()
			go func() {
				time.Sleep(1 * time.Second)
				resultsQueue <- resultpair{token.String(), "REAL_VALUE!"}
			}()
			return "{{ getOutput \"" + token.String() + "\" }}"
		},
	}

	return &totalJobs, funcMap
}

func getFuncMapNested(ctx context.Context, output map[string]string) template.FuncMap {
	var funcMapNested = template.FuncMap{
		"getOutput": func(input string) string { return output[input] },
	}
	return funcMapNested
}

func main() {
	initial := "{{sleep}} {{sleep}} {{sleep}}"

	resultsQueue := make(chan resultpair)
	outputQueue := make(chan map[string]string)
	// totalJobs is decieving: only ever accessed by one thread at a time, so shouldn't need locking (I think)
	totalJobs, funcMap := getFuncMap(context.TODO(), resultsQueue)

	fmt.Printf("About to execute first template: %s\n", initial)
	fmt.Printf("TOTAL JOBS: %d\n", *totalJobs)
	tmpl, _ := template.New("test").Funcs(funcMap).Parse(initial)
	var buf bytes.Buffer
	tmpl.Execute(&buf, nil)
	fmt.Printf("Got translated template: %s\n", buf.String())
	fmt.Printf("TOTAL JOBS: %d\n", *totalJobs)

	go func(totalJobs *int) {
		var results map[string]string
		results = make(map[string]string)

		for i := 0; i < *totalJobs; i++ {
			res := <-resultsQueue
			results[res.key] = res.value
		}
		outputQueue <- results
		close(outputQueue)
	}(totalJobs)

	output := <-outputQueue
	close(resultsQueue)
	fmt.Printf("Output of the goroutine: %s\n", output)

	funcMapNested := getFuncMapNested(context.TODO(), output)
	tmpl2, _ := template.New("nested").Funcs(funcMapNested).Parse(buf.String())
	var buf2 bytes.Buffer
	tmpl2.Execute(&buf2, nil)

	fmt.Printf("results: %s\n", buf2.String())
}
