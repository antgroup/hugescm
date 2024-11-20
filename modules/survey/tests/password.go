//go:build ignore

package main

import (
	"github.com/antgroup/hugescm/modules/survey"
	TestUtil "github.com/antgroup/hugescm/modules/survey/tests/util"
)

var value = ""

var table = []TestUtil.TestTableEntry{
	{
		"standard", &survey.Password{Message: "Please type your password:"}, &value, nil,
	},
	{
		"please make sure paste works", &survey.Password{Message: "Please paste your password:"}, &value, nil,
	},
	{
		"no help, send '?'", &survey.Password{Message: "Please type your password:"}, &value, nil,
	},
}

func main() {
	TestUtil.RunTable(table)
}
