package samplebackend

import (
    "time"
    "text/template"
)

var funcMap = template.FuncMap {
    "sleep": func() string { time.Sleep(1 * time.Second); return "slept" },
    "hello": func() string { return "HELLO WORLD" },
}
