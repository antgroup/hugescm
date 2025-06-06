//go:build ignore

package main

import (
	"fmt"

	"github.com/antgroup/hugescm/modules/survey"
)

var simpleQs = []*survey.Question{
	{
		Name: "color",
		Prompt: &survey.Select{
			Message: "select1:",
			Options: []string{"red", "blue", "green"},
		},
		Validate: survey.Required,
	},
	{
		Name: "color2",
		Prompt: &survey.Select{
			Message: "select2:",
			Options: []string{"red", "blue", "green"},
		},
		Validate: survey.Required,
	},
}

func main() {
	answers := struct {
		Color  string
		Color2 string
	}{}
	// ask the question
	err := survey.Ask(simpleQs, &answers)

	if err != nil {
		fmt.Println(err.Error())
		return
	}
	// print the answers
	fmt.Printf("%s and %s.\n", answers.Color, answers.Color2)
}
