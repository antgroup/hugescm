package survey

import (
	"strings"
)

/*
Password is like a normal Input but the text shows up as *'s and there is no default. Response
type is a string.

	password := ""
	prompt := &survey.Password{ Message: "Please type your password" }
	survey.AskOne(prompt, &password)
*/
type Password struct {
	Renderer
	Message string
	Help    string
	answer  string
}

type PasswordTemplateData struct {
	Password
	ShowHelp   bool
	Config     *PromptConfig
	ShowAnswer bool
	Answer     string
}

// PasswordQuestionTemplate is a template with color formatting. See Documentation: https://github.com/mgutz/ansi#style-format
var PasswordQuestionTemplate = `
{{- if .ShowHelp }}{{- color .Config.Icons.Help.Format }}{{ .Config.Icons.Help.Text }} {{ .Help }}{{color "reset"}}{{"\n"}}{{end}}
{{- color .Config.Icons.Question.Format }}{{ .Config.Icons.Question.Text }} {{color "reset"}}
{{- color "default+hb"}}{{ .Message }} {{color "reset"}}
{{- if .ShowAnswer}}
  {{- color "cyan"}}{{.Answer}}{{color "reset"}}{{"\n"}}
{{- else }}
  {{- if and .Help (not .ShowHelp)}}{{color "cyan"}}[{{ .Config.HelpInput }} for help]{{color "reset"}} {{end}}
{{- end }}`

func (p *Password) Prompt(config *PromptConfig) (any, error) {
	// render the question template
	err := p.Render(
		PasswordQuestionTemplate,
		PasswordTemplateData{
			Password: *p,
			Config:   config,
		},
	)
	if err != nil {
		return nil, err
	}

	rr := p.NewRuneReader()
	_ = rr.SetTermMode()
	defer func() {
		_ = rr.RestoreTermMode()
	}()
	cursor := p.NewCursor()

	// no help msg?  Just return any response
	if p.Help == "" {
		line, err := rr.ReadLine(config.HideCharacter)
		p.answer = string(line)
		if err != nil {
			return p.answer, err
		}
		_ = cursor.PreviousLine(1)
		p.AppendRenderedText(strings.Repeat(string(config.HideCharacter), len(p.answer)))
		return p.answer, err
	}

	var line []rune
	// process answers looking for help prompt answer
	for {
		line, err = rr.ReadLine(config.HideCharacter)
		p.answer = string(line)
		if err != nil {
			return p.answer, err
		}

		if p.answer == config.HelpInput {
			// terminal will echo the \n so we need to jump back up one row
			_ = cursor.PreviousLine(1)

			err = p.Render(
				PasswordQuestionTemplate,
				PasswordTemplateData{
					Password: *p,
					ShowHelp: true,
					Config:   config,
				},
			)
			if err != nil {
				return "", err
			}
			continue
		}

		break
	}
	p.AppendRenderedText(strings.Repeat(string(config.HideCharacter), len(p.answer)))
	_ = cursor.PreviousLine(1)
	return p.answer, err
}

// Cleanup re-generates the input as the hide character
func (prompt *Password) Cleanup(config *PromptConfig, val any) error {
	return prompt.Render(
		PasswordQuestionTemplate,
		PasswordTemplateData{
			Password:   *prompt,
			ShowHelp:   false,
			Config:     config,
			ShowAnswer: true,
			Answer:     strings.Repeat(string(config.HideCharacter), len(prompt.answer)),
		})
}
