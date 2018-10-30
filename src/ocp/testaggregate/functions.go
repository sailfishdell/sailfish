package testaggregate

import (
	"github.com/Knetic/govaluate"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
)

var functions map[string]govaluate.ExpressionFunction

func init() {
	// share our functions with awesome mapper
	functions = awesome_mapper2.InitFunctions()

	awesome_mapper2.AddFunction("array",
		func(args ...interface{}) (interface{}, error) {
			a := []interface{}{}
			for _, i := range args {
				a = append(a, i)
			}
			return a, nil
		})

	awesome_mapper2.AddFunction("add_attribute_property",
		func(args ...interface{}) (interface{}, error) {
			return map[string]map[string]map[string]interface{}{}, nil
		})
}
