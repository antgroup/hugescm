package survey

import (
	"fmt"

	"github.com/antgroup/hugescm/modules/survey/core"
	"github.com/antgroup/hugescm/modules/survey/terminal"
)

/*
Info formats an info message. Response type is a nil pointer.

	prompt := &survey.Info{ Message: "The weather is nice today" }
	survey.AskOne(prompt, nil)
*/
type Info struct {
	Renderer
	Message string
}

type InfoTemplateData struct {
	Info
	Config *PromptConfig
}

// InfoTemplate is a template with color formatting. See Documentation: https://github.com/mgutz/ansi#style-format
var InfoTemplate = `
{{- color .Config.Icons.Info.Format }}{{ .Config.Icons.Info.Text }} {{color "reset"}}
{{- color "default+hb"}}{{ .Message }}{{color "reset"}}{{"\n"}}`

func (i *Info) Prompt(config *PromptConfig) (any, error) {
	// render the template
	userOut, _, err := core.RunTemplate(
		InfoTemplate,
		InfoTemplateData{
			Info:   *i,
			Config: config,
		},
	)
	if err != nil {
		return "", err
	}

	_, err = fmt.Fprint(terminal.NewAnsiStdout(i.Stdio().Out), userOut)
	return nil, err
}

// Cleanup hides the string with a fixed number of characters.
func (prompt *Info) Cleanup(config *PromptConfig, val any) error {
	return nil
}
