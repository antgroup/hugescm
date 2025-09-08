package core

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"text/template"

	"github.com/antgroup/hugescm/modules/term"
	"github.com/mgutz/ansi"
)

// DisableColor can be used to make testing reliable
var DisableColor = false

var TemplateFuncsWithColor = map[string]any{
	// Templates with Color formatting. See Documentation: https://github.com/mgutz/ansi#style-format
	"color":  color,
	"spaces": spaces,
}

func color(style string) string {
	switch style {
	case "gray":
		// Fails on windows, only affects defaults
		if term.StdoutLevel >= term.Level256 {
			return ansi.ColorCode("8")
		}
		return ansi.ColorCode("default")
	default:
		return ansi.ColorCode(style)
	}
}

func spaces(selectorText string) string {
	length := 0
	for _, s := range selectorText {
		if len(string(s)) == 1 {
			// This character is displayed as one space
			length++
		} else {
			// This character is displayed differently than its length : emoji
			length += 2
		}
	}
	return strings.Repeat(" ", length)
}

var TemplateFuncsNoColor = map[string]any{
	// Templates without Color formatting. For layout/ testing.
	"color": func(color string) string {
		return ""
	},
	"spaces": spaces,
}

// envColorDisabled returns if output colors are forbid by environment variables
func envColorDisabled() bool {
	return os.Getenv("NO_COLOR") != "" || os.Getenv("CLICOLOR") == "0"
}

// envColorForced returns if output colors are forced from environment variables
func envColorForced() bool {
	val, ok := os.LookupEnv("CLICOLOR_FORCE")
	return ok && val != "0"
}

// RunTemplate returns two formatted strings given a template and
// the data it requires. The first string returned is generated for
// user-facing output and may or may not contain ANSI escape codes
// for colored output. The second string does not contain escape codes
// and can be used by the renderer for layout purposes.
func RunTemplate(tmpl string, data any) (string, string, error) {
	tPair, err := GetTemplatePair(tmpl)
	if err != nil {
		return "", "", err
	}
	userBuf := bytes.NewBufferString("")
	err = tPair[0].Execute(userBuf, data)
	if err != nil {
		return "", "", err
	}
	layoutBuf := bytes.NewBufferString("")
	err = tPair[1].Execute(layoutBuf, data)
	if err != nil {
		return userBuf.String(), "", err
	}
	return userBuf.String(), layoutBuf.String(), err
}

var (
	memoizedGetTemplate = map[string][2]*template.Template{}

	memoMutex = &sync.RWMutex{}
)

// GetTemplatePair returns a pair of compiled templates where the
// first template is generated for user-facing output and the
// second is generated for use by the renderer. The second
// template does not contain any color escape codes, whereas
// the first template may or may not depending on DisableColor.
func GetTemplatePair(tmpl string) ([2]*template.Template, error) {
	memoMutex.RLock()
	if t, ok := memoizedGetTemplate[tmpl]; ok {
		memoMutex.RUnlock()
		return t, nil
	}
	memoMutex.RUnlock()

	templatePair := [2]*template.Template{nil, nil}

	templateNoColor, err := template.New("prompt").Funcs(TemplateFuncsNoColor).Parse(tmpl)
	if err != nil {
		return [2]*template.Template{}, err
	}

	templatePair[1] = templateNoColor

	envColorHide := envColorDisabled() && !envColorForced()
	if DisableColor || envColorHide {
		templatePair[0] = templatePair[1]
	} else {
		templateWithColor, err := template.New("prompt").Funcs(TemplateFuncsWithColor).Parse(tmpl)
		templatePair[0] = templateWithColor
		if err != nil {
			return [2]*template.Template{}, err
		}
	}

	memoMutex.Lock()
	memoizedGetTemplate[tmpl] = templatePair
	memoMutex.Unlock()
	return templatePair, nil
}
