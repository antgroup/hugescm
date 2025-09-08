package survey

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/antgroup/hugescm/modules/survey/core"
	"github.com/antgroup/hugescm/modules/survey/terminal"
)

func init() {
	// disable color output for all prompts to simplify testing
	core.DisableColor = true
}

func TestSliderRender(t *testing.T) {

	prompt := Slider{
		Message: "Pick your number:",
		Max:     50,
		Default: 40,
	}

	helpfulPrompt := prompt
	helpfulPrompt.Help = "This is helpful"

	tests := []struct {
		title        string
		prompt       Slider
		promptOption AskOpt
		data         SliderTemplateData
		expected     string
	}{
		{
			"Test Slider question output",
			prompt,
			nil,
			SliderTemplateData{SelectedValue: 3},
			strings.Join(
				[]string{
					fmt.Sprintf("%s Pick your number: [Use side arrows to alter value by 1, vertical arrows to alter value by 10]", defaultIcons().Question.Text),
					fmt.Sprintf("  %s%s%s", defaultIcons().SliderFiller.Text, defaultIcons().SliderCursor.Text, strings.Repeat(defaultIcons().SliderFiller.Text, 23)),
				},
				"\n",
			),
		},
		{
			"Test Slider answer output",
			prompt,
			nil,
			SliderTemplateData{SelectedValue: 15, ShowAnswer: true},
			fmt.Sprintf("%s Pick your number: 15\n", defaultIcons().Question.Text),
		},
		{
			"Test Slider question output with help hidden",
			helpfulPrompt,
			nil,
			SliderTemplateData{SelectedValue: 3},
			strings.Join(
				[]string{
					fmt.Sprintf("%s Pick your number: [Use side arrows to alter value by 1, vertical arrows to alter value by 10, %s for more help]", defaultIcons().Question.Text, string(defaultPromptConfig().HelpInput)),
					fmt.Sprintf("  %s%s%s", defaultIcons().SliderFiller.Text, defaultIcons().SliderCursor.Text, strings.Repeat(defaultIcons().SliderFiller.Text, 23)),
				},
				"\n",
			),
		},
		{
			"Test Slider question output with help shown",
			helpfulPrompt,
			nil,
			SliderTemplateData{SelectedValue: 3, ShowHelp: true},
			strings.Join(
				[]string{
					fmt.Sprintf("%s This is helpful", defaultIcons().Help.Text),
					fmt.Sprintf("%s Pick your number: [Use side arrows to alter value by 1, vertical arrows to alter value by 10]", defaultIcons().Question.Text),
					fmt.Sprintf("  %s%s%s", defaultIcons().SliderFiller.Text, defaultIcons().SliderCursor.Text, strings.Repeat(defaultIcons().SliderFiller.Text, 23)),
				},
				"\n",
			),
		},
	}

	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			r, w, err := os.Pipe()
			assert.NoError(t, err)

			test.prompt.WithStdio(terminal.Stdio{Out: w})
			test.data.Slider = test.prompt

			options := defaultAskOptions()
			if test.promptOption != nil {
				err = test.promptOption(options)
				assert.NoError(t, err)
			}
			// set the icon set
			test.data.Config = &options.PromptConfig

			// Compute the state
			test.prompt.MaxSize = 25
			test.prompt.selectedValue = test.data.SelectedValue
			test.data.ChangeInterval = 10
			test.data.SliderContent = test.prompt.computeSliderContent(test.data.Config)

			err = test.prompt.Render(
				SliderQuestionTemplate,
				test.data,
			)
			assert.NoError(t, err)

			assert.NoError(t, w.Close())
			var buf bytes.Buffer
			_, err = io.Copy(&buf, r)
			assert.NoError(t, err)

			assert.Contains(t, buf.String(), test.expected)
		})
	}
}

func TestSliderPrompt(t *testing.T) {

	//Overflow, min max custom, up arrows, help
	tests := []PromptTest{
		{
			"basic interaction: 1",
			&Slider{
				Message: "Choose a number:",
				Max:     25,
			},
			nil,
			func(c expectConsole) {
				c.ExpectString("*------------------------- 0")
				// Increase the value by one
				c.Send(string(terminal.KeyArrowRight))
				c.ExpectString("-*------------------------ 1")
				// Validate
				c.SendLine("")
				c.ExpectEOF()
			},
			1,
		},
		{
			"basic interaction: default",
			&Slider{
				Message: "Choose a number:",
				Default: 20,
				Max:     25,
			},
			nil,
			func(c expectConsole) {
				c.ExpectString("--------------------*----- 20")
				// Decrease the value by one
				c.Send(string(terminal.KeyArrowLeft))
				c.ExpectString("-------------------*------ 19")
				// Validate
				c.SendLine("")
				c.ExpectEOF()
			},
			19,
		},
		{
			"basic interaction: custom min and max",
			&Slider{
				Message: "Choose a number:",
				Min:     20,
				Max:     70,
			},
			nil,
			func(c expectConsole) {
				c.ExpectString("*------------------------- 20")
				// Decrease the value by one - does nothing
				c.Send(string(terminal.KeyArrowLeft))
				c.ExpectString("*------------------------- 20")
				// Validate
				c.SendLine("")
				c.ExpectEOF()
			},
			20,
		},
		{
			"basic interaction: custom min and max",
			&Slider{
				Message: "Choose a number:",
				Min:     -70,
				Max:     -20,
			},
			nil,
			func(c expectConsole) {
				c.ExpectString("*------------------------- -70")
				// Decrease the value by 10 - does nothing
				c.Send(string(terminal.KeyArrowDown))
				c.ExpectString("*------------------------- -70")
				// Validate
				c.SendLine("")
				c.ExpectEOF()
			},
			-70,
		},
		{
			"basic interaction: overflow max",
			&Slider{
				Message: "Choose a number:",
				Max:     10,
			},
			nil,
			func(c expectConsole) {
				c.ExpectString("*---------- 0")
				// Increase the value by 10
				c.Send(string(terminal.KeyArrowUp))
				c.ExpectString("----------* 10")
				// Increase the value by 10 - does nothing
				c.Send(string(terminal.KeyArrowUp))
				c.ExpectString("----------* 10")
				// Increase the value by 1 - does nothing
				c.Send(string(terminal.KeyArrowRight))
				c.ExpectString("----------* 10")
				// Validate
				c.SendLine("")
				c.ExpectEOF()
			},
			10,
		},
	}

	for _, test := range tests {
		testName := strings.TrimPrefix(test.name, "SKIP: ")
		t.Run(testName, func(t *testing.T) {
			if testName != test.name {
				t.Skipf("warning: flakey test %q", testName)
			}
			RunPromptTest(t, test)
		})
	}
}

func TestSliderError(t *testing.T) {
	// Error should be sent on:
	// - max < min
	// - max == min
	// - default outside min max interval

	invalidSliders := []Slider{
		{Max: 10, Min: 50},
		{Max: 10, Min: 10},
		{Max: 15, Min: 10, Default: -40},
	}

	for _, slider := range invalidSliders {
		var output int
		err := Ask([]*Question{{Prompt: &slider}}, &output)
		// if we didn't get an error
		if err == nil {
			// the test failed
			t.Errorf("Did not encounter error when asking whith invalid slider: %s", fmt.Sprint(slider))
		}
	}
}
