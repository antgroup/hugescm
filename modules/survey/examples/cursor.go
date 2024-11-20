//go:build ignore

package main

import (
	"fmt"

	"github.com/antgroup/hugescm/modules/survey"
)

// the questions to ask
var simpleQs = []*survey.Question{
	{
		Name: "name",
		Prompt: &survey.Input{
			Message: "What is your name?",
		},
		Validate: survey.Required,
	},
}

func main() {
	ansmap := make(map[string]any)

	// ask the question
	err := survey.Ask(simpleQs, &ansmap, survey.WithShowCursor(true))

	if err != nil {
		fmt.Println(err.Error())
		return
	}
	// print the answers
	fmt.Printf("Your name is %s.\n", ansmap["name"])
}
