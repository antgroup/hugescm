package kong

import (
	"fmt"
	"regexp"
	"strings"
)

var interpolationRegex = regexp.MustCompile(`(\$\$)|((?:\${([[:alpha:]_][[:word:]]*))(?:=([^}]+))?})|(\$)|([^$]+)`)

// HasInterpolatedVar returns true if the variable "v" is interpolated in "s".
func HasInterpolatedVar(s string, v string) bool {
	matches := interpolationRegex.FindAllStringSubmatch(s, -1)
	for _, match := range matches {
		if name := match[3]; name == v {
			return true
		}
	}
	return false
}

// Interpolate variables from vars into s for substrings in the form ${var} or ${var=default}.
func interpolate(s string, vars Vars, updatedVars map[string]string) (string, error) {
	var out strings.Builder
	matches := interpolationRegex.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return s, nil
	}
	// Clone vars with updatedVars if there are any updates
	if len(updatedVars) > 0 {
		vars = vars.CloneWith(updatedVars)
	}
	for _, match := range matches {
		if dollar := match[1]; dollar != "" {
			out.WriteString("$")
		} else if name := match[3]; name != "" {
			value, ok := vars[name]
			if !ok {
				// No default value.
				if match[4] == "" {
					return "", fmt.Errorf("undefined variable ${%s}", name)
				}
				value = match[4]
			}
			out.WriteString(value)
		} else {
			out.WriteString(match[0])
		}
	}
	return out.String(), nil
}
