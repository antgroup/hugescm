//go:build ignore

package main

import (
	"github.com/antgroup/hugescm/modules/survey"
	TestUtil "github.com/antgroup/hugescm/modules/survey/tests/util"
)

var val = ""

var table = []TestUtil.TestTableEntry{
	{
		"no default", &survey.Input{Message: "Hello world"}, &val, nil,
	},
	{
		"default", &survey.Input{Message: "Hello world", Default: "default"}, &val, nil,
	},
	{
		"no help, send '?'", &survey.Input{Message: "Hello world"}, &val, nil,
	},
	{
		"Home, End Button test in random location", &survey.Input{Message: "Hello world"}, &val, nil,
	}, {
		"Delete and forward delete test at random location (test if screen overflows)", &survey.Input{Message: "Hello world"}, &val, nil,
	}, {
		"Moving around lines with left & right arrow keys", &survey.Input{Message: "Hello world"}, &val, nil,
	}, {
		"Runes with width > 1. Enter 一 you get to the next line", &survey.Input{Message: "Hello world"}, &val, nil,
	},
}

func main() {
	TestUtil.RunTable(table)
}
