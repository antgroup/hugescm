package survey

import (
	"errors"

	"github.com/antgroup/hugescm/modules/survey/terminal"
)

/*
Slider is a prompt that presents a slider that can be manipulated
using the arrow keys and enter. Response type is an int.

	var count int
	prompt := &survey.Slider{
		Message: "Choose a number:",
		Max: 50
	}
	survey.AskOne(prompt, &count)
*/
type Slider struct {
	Renderer
	Message        string
	Default        int
	Help           string
	Min            int
	Max            int
	ChangeInterval int
	MaxSize        int
	selectedValue  int
	showingHelp    bool
}

// SliderTemplateData is the data available to the templates when processing
type SliderTemplateData struct {
	Slider
	SelectedValue int
	ShowAnswer    bool
	ShowHelp      bool
	SliderContent []Icon
	Config        *PromptConfig
}

var SliderQuestionTemplate = `
{{- define "sliderValue"}}{{- color $.Format}}{{ $.Text}}{{- color "reset"}}{{end}}
{{- if .ShowHelp }}{{- color .Config.Icons.Help.Format }}{{ .Config.Icons.Help.Text }} {{ .Help }}{{color "reset"}}{{- "\n"}}{{end}}
{{- color .Config.Icons.Question.Format }}{{ .Config.Icons.Question.Text }} {{color "reset"}}
{{- color "default+hb"}}{{ .Message }} {{color "reset"}}
{{- if .ShowAnswer}}
  {{- color "cyan"}}{{.SelectedValue}}{{color "reset"}}{{"\n"}}
{{- else}}
  {{- color "cyan"}}[Use side arrows to alter value by 1, vertical arrows to alter value by {{ .ChangeInterval }}{{- if and .Help (not .ShowHelp)}}, {{ .Config.HelpInput }} for more help{{end}}]{{color "reset"}}{{- "\n"}}
  {{- "  "}}{{- range $_, $option := .SliderContent}}{{- template "sliderValue" $option}}{{- end}} {{ .SelectedValue }}{{- "\n"}}
{{- end}}`

func (s *Slider) changeValue(change int) {
	s.selectedValue += change
	if change > 0 && s.selectedValue > s.Max {
		s.selectedValue = s.Max
	} else if s.selectedValue < s.Min {
		s.selectedValue = s.Min
	}

}

// OnChange is called on every keypress.
func (s *Slider) OnChange(key rune, config *PromptConfig) bool {

	// if the user pressed the enter key and the index is a valid option
	if key == terminal.KeyEnter || key == '\n' {
		return true

		// if the user pressed the up arrow
	} else if key == terminal.KeyArrowUp && s.selectedValue < s.Max {
		s.changeValue(s.ChangeInterval)
		// if the user pressed down
	} else if key == terminal.KeyArrowDown && s.selectedValue > s.Min {
		s.changeValue(-s.ChangeInterval)
		// only show the help message if we have one
	} else if string(key) == config.HelpInput && s.Help != "" {
		s.showingHelp = true
		// if the user wants to decrease the value by one
	} else if key == terminal.KeyArrowLeft {
		s.changeValue(-1)
		// if the user wants to increase the value by one
	} else if key == terminal.KeyArrowRight {
		s.changeValue(1)
	}

	tmplData := SliderTemplateData{
		Slider:        *s,
		SelectedValue: s.selectedValue,
		ShowHelp:      s.showingHelp,
		Config:        config,
		SliderContent: s.computeSliderContent(config),
	}

	// render the options
	_ = s.Render(SliderQuestionTemplate, tmplData)

	// keep prompting
	return false
}

type SliderContent struct {
	Format string
	Value  string
}

func (s *Slider) computeSliderContent(config *PromptConfig) []Icon {
	var output []Icon

	// Computing how much one character represents
	interval := (s.Max - s.Min) / s.MaxSize
	if interval <= 0 {
		interval = 1
	}
	for i := s.Min; i <= s.Max; i += interval {
		if s.selectedValue >= i && s.selectedValue < i+interval {
			// Our selected value is in this range
			output = append(output, config.Icons.SliderCursor)
		} else {
			output = append(output, config.Icons.SliderFiller)
		}
	}
	return output
}

func (s *Slider) Prompt(config *PromptConfig) (any, error) {
	// if configuration is incoherent
	if s.Max <= s.Min {
		// we failed
		return "", errors.New("please provide an interval")
	}
	// This is only so that user changing min max do not always have to change default accordingly
	if s.Default == 0 && (s.Min > 0 || s.Max <= 0) {
		s.Default = s.Min
	}
	if s.Default > s.Max || s.Default < s.Min {
		// we failed
		return "", errors.New("default value outside range")
	}
	s.selectedValue = s.Default
	if s.ChangeInterval == 0 {
		s.ChangeInterval = 10
	}
	if s.MaxSize == 0 {
		s.MaxSize = 25
	}

	cursor := s.NewCursor()
	_ = cursor.Hide() // hide the cursor
	defer func() {
		_ = cursor.Show() // show the cursor when we're done
	}()

	tmplData := SliderTemplateData{
		Slider:        *s,
		SelectedValue: s.selectedValue,
		ShowHelp:      s.showingHelp,
		Config:        config,
		SliderContent: s.computeSliderContent(config),
	}

	// ask the question
	err := s.Render(SliderQuestionTemplate, tmplData)
	if err != nil {
		return "", err
	}

	rr := s.NewRuneReader()
	_ = rr.SetTermMode()
	defer func() {
		_ = rr.RestoreTermMode()
	}()

	// start waiting for input
	for {
		r, _, err := rr.ReadRune()
		if err != nil {
			return "", err
		}
		if r == terminal.KeyInterrupt {
			return "", terminal.ErrInterrupt
		}
		if r == terminal.KeyEndTransmission {
			break
		}
		if s.OnChange(r, config) {
			break
		}
	}
	return s.selectedValue, err
}

func (s *Slider) Cleanup(config *PromptConfig, val any) error {
	return s.Render(
		SliderQuestionTemplate,
		SliderTemplateData{
			Slider:        *s,
			SelectedValue: s.selectedValue,
			ShowHelp:      false,
			ShowAnswer:    true,
			Config:        config,
		},
	)
}
