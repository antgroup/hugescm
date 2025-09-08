package survey

import (
	"fmt"
	"strings"
	"testing"

	"github.com/antgroup/hugescm/modules/survey/core"
	"github.com/stretchr/testify/assert"
)

func init() {
	// disable color output for all prompts to simplify testing
	core.DisableColor = true
}

func TestInfoRender(t *testing.T) {

	tests := []struct {
		title    string
		prompt   Info
		data     InfoTemplateData
		expected string
	}{
		{
			"Test Info Formatting",
			Info{Message: "The weather is nice today"},
			InfoTemplateData{},
			fmt.Sprintf("%s The weather is nice today\n", defaultIcons().Info.Text),
		},
	}

	for _, test := range tests {
		test.data.Info = test.prompt

		// set the icon set
		test.data.Config = defaultPromptConfig()

		actual, _, err := core.RunTemplate(
			InfoTemplate,
			&test.data,
		)
		assert.Nil(t, err, test.title)
		assert.Equal(t, test.expected, actual, test.title)
	}
}

func TestInfoPrompt(t *testing.T) {
	tests := []PromptTest{
		{
			"SKIP: Test Info prompt interaction",
			&Info{
				Message: "A valuable information",
			},
			nil,
			func(c expectConsole) {
				c.ExpectString("A valuable information")
				c.ExpectEOF()
			},
			nil,
		},
	}
	for _, test := range tests {
		testName := strings.TrimPrefix(test.name, "SKIP: ")
		t.Run(test.name, func(t *testing.T) {
			if testName != test.name {
				t.Skipf("warning: flakey test %q", testName)
			}
			RunPromptTest(t, test)
		})
	}
}
