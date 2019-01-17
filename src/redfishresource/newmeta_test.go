package domain

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParsing(t *testing.T) {
	var tests = []struct {
		testname string
		input    *RedfishResourceProperty
		expected interface{}
	}{
		{"test nil", &RedfishResourceProperty{Value: nil}, nil},
		{"test string 1", &RedfishResourceProperty{Value: "happy"}, "happy"},
		{"test int 1", &RedfishResourceProperty{Value: 42}, 42},
		{"test string 2", &RedfishResourceProperty{Value: "joy"}, "joy"},
		{"test float 1", &RedfishResourceProperty{Value: 1.02}, 1.02},
		{"test string 3", &RedfishResourceProperty{Value: "happy"}, "happy"},
		{"test slice 1", &RedfishResourceProperty{Value: []string{"happy joy"}}, []string{"happy joy"}},
		{"test map 1", &RedfishResourceProperty{Value: map[string]string{"happy": "joy"}}, map[string]string{"happy": "joy"}},
		{"test recursion 1",
			&RedfishResourceProperty{Value: map[string]interface{}{"happy": &RedfishResourceProperty{Value: "joy"}}},
			map[string]interface{}{"happy": "joy"}},
		{"test nil map value 1",
			&RedfishResourceProperty{Value: map[string]interface{}{"happy": nil}},
			map[string]interface{}{"happy": nil}},
	}
	for _, subtest := range tests {
		t.Run(subtest.testname, func(t *testing.T) {
			output, _ := ProcessGET(context.Background(), subtest.input, nil)
			assert.EqualValues(t, subtest.expected, output)
		})
	}
}
