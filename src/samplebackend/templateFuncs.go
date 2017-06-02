package samplebackend

import (
	"text/template"
)

var funcMap = template.FuncMap{
	"hello": func() string { return "HELLO WORLD" },
}
