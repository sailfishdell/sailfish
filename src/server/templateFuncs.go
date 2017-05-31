package server

import (
    "time"
    "text/template"
)

var funcMap = template.FuncMap {
    "sleep": func() string { time.Sleep(1 * time.Second); return "slept" },
    "hello": func() string { return "HELLO WORLD" },
}

// wrapper around template.New()
func New(name string) *template.Template {
    return template.New(name).Funcs(funcMap)
}

func InjectFunctions(funcs map[string]interface{}) {
    for k,v := range funcMap {
        funcs[k] = v
    }
}
