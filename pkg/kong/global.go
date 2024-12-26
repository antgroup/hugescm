package kong

import (
	"os"
)

// Parse constructs a new parser and parses the default command-line.
func Parse(cli any, options ...Option) *Context {
	parser, err := New(cli, options...)
	if err != nil {
		panic(err)
	}
	ctx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)
	return ctx
}

func ParseArgs(cli any, arsg []string, options ...Option) *Context {
	parser, err := New(cli, options...)
	if err != nil {
		panic(err)
	}
	ctx, err := parser.Parse(arsg)
	parser.FatalIfErrorf(err)
	return ctx
}
