package testaggregate

import (
	"github.com/Knetic/govaluate"
)

var functions map[string]govaluate.ExpressionFunction

func init() {
	functions = map[string]govaluate.ExpressionFunction{
		"array": func(args ...interface{}) (interface{}, error) {
			a := []interface{}{}
			for _, i := range args {
				a = append(a, i)
			}
			return a, nil
		},
		"add_attribute_property": func(args ...interface{}) (interface{}, error) {
			return map[string]map[string]map[string]interface{}{}, nil
		},
	}
}
