package view

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/superchalupa/sailfish/src/log15adapter"
	"github.com/superchalupa/sailfish/src/ocp/model"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func init() {
	log15adapter.InitializeApplicationLogging("")
}

func TestView(t *testing.T) {
	testModel := model.New(model.UpdateProperty("test", "HAPPY"))
	_ = New(
		WithURI("TESTURI"),
		WithModel("default", testModel),
	)

	var tests = []*struct {
		testname string
		input    *domain.RedfishResourceProperty
		expected interface{}
	}{
		{"test string 1", &domain.RedfishResourceProperty{Value: "appy", Meta: map[string]interface{}{
			"GET": map[string]interface{}{"plugin": "TESTURI", "property": "test", "model": "default"}}}, "HAPPY"},
		{"test recursion 1",
			&domain.RedfishResourceProperty{Value: map[string]interface{}{"happy": &domain.RedfishResourceProperty{Value: "joy"}}},
			map[string]interface{}{"happy": "joy"}},
	}
	for _, subtest := range tests {
		t.Run(subtest.testname, func(t *testing.T) {
			domain.NewGet(context.Background(), nil, subtest.input, &domain.RedfishAuthorizationProperty{})
			output := domain.Flatten(subtest.input.Value)

			assert.EqualValues(t, subtest.expected, output)
		})
	}
}
