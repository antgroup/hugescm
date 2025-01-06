package cli

import (
	"fmt"
	"os"
	"testing"

	"github.com/antgroup/hugescm/pkg/kong"
	"github.com/antgroup/hugescm/pkg/tr"
	"github.com/antgroup/hugescm/pkg/version"
)

type Checkout struct {
	UnresolvedArgs  []string `arg:"" optional:""`
	Branch          string   `name:"branch" short:"b" help:"Direct the new HEAD to the <name> branch after checkout"`
	TagName         string   `name:"tag" short:"t" help:"Direct the new HEAD to the <name> tag's commit after checkout"`
	Commit          string   `name:"commit" help:"Direct the new HEAD to the <commit> branch after checkout"`
	Sparse          []string `name:"sparse" short:"s" help:"A subset of repository files, all files are checked out by default" type:"string"`
	Limit           int64    `name:"limit" short:"L" help:"Omits blobs larger than n bytes or units. n may be zero. supported units: KB,MB,GB,K,M,G" default:"-1" type:"size"`
	Batch           bool     `name:"batch" help:"Get and checkout files for each provided on stdin"`
	Snapshot        bool     `name:"snapshot" help:"Checkout a non-editable snapshot"`
	Depth           int      `name:"depth" help:"Create a shallow clone with a history truncated to the specified number of commits" default:"1"`
	One             bool     `name:"one" help:"Checkout large files one after another"`
	Quiet           bool     `name:"quiet" help:"Operate quietly. Progress is not reported to the standard error stream"`
	passthroughArgs []string `kong:"-"`
}

func (c *Checkout) Passthrough(paths []string) {
	c.passthroughArgs = append(c.passthroughArgs, paths...)
}

func (c *Checkout) Run() error {
	fmt.Fprintf(os.Stderr, "unresolvedArgs: %v passthroughArgs: %v\n", c.UnresolvedArgs, c.passthroughArgs)
	return nil
}

type Diff struct {
	NoIndex         bool     `name:"no-index" help:"Compares two given paths on the filesystem"`
	NameOnly        bool     `name:"name-only" help:"Show only names of changed files"`
	NameStatus      bool     `name:"name-status" help:"Show names and status of changed files"`
	Numstat         bool     `name:"numstat" help:"Show numeric diffstat instead of patch"`
	Stat            bool     `name:"stat" help:"Show diffstat instead of patch"`
	Shortstat       bool     `name:"shortstat" help:"Output only the last line of --stat format"`
	Z               bool     `short:"z" shortonly:"" help:"Output diff-raw with lines terminated with NUL"`
	Staged          bool     `name:"staged" help:"Compare the differences between the staging area and <revision>"`
	Cached          bool     `name:"cached" help:"Compare the differences between the staging area and <revision>"`
	Textconv        bool     `name:"textconv" help:"Converting text to Unicode"`
	MergeBase       string   `name:"merge-base" help:"If --merge-base is given, use the common ancestor of <commit> and HEAD instead"`
	Histogram       bool     `name:"histogram" help:"Generate a diff using the \"Histogram diff\" algorithm"`
	ONP             bool     `name:"onp" help:"Generate a diff using the \"O(NP) diff\" algorithm"`
	Myers           bool     `name:"myers" help:"Generate a diff using the \"Myers diff\" algorithm"`
	Patience        bool     `name:"patience" help:"Generate a diff using the \"Patience diff\" algorithm"`
	Minimal         bool     `name:"minimal" help:"Spend extra time to make sure the smallest possible diff is produced"`
	DiffAlgorithm   string   `name:"diff-algorithm" help:"Choose a diff algorithm, supported: histogram|onp|myers|patience|minimal" placeholder:"<algorithm>"`
	Output          string   `name:"output" help:"Output to a specific file instead of stdout" placeholder:"<file>"`
	From            string   `arg:"" optional:"" name:"from" help:""`
	To              string   `arg:"" optional:"" name:"to" help:""`
	passthroughArgs []string `kong:"-"`
}

func (c *Diff) Passthrough(paths []string) {
	c.passthroughArgs = append(c.passthroughArgs, paths...)
}

func (c *Diff) Run() error {
	fmt.Fprintf(os.Stderr, "from {%s} to {%s} args: %v\n", c.From, c.To, c.passthroughArgs)
	return nil
}

type App struct {
	Checkout Checkout `cmd:"" name:"co" help:"checkout"`
	Diff     Diff     `cmd:"" name:"diff" help:"diff"`
}

func TestCheckout(t *testing.T) {
	parseArgs := func(args []string) {
		var app App
		ctx := kong.ParseArgs(&app, args,
			kong.Name("zeta"),
			kong.Description(tr.W("HugeSCM - A next generation cloud-based version control system")),
			kong.UsageOnError(),
			kong.ConfigureHelp(kong.HelpOptions{
				Compact:             true,
				NoExpandSubcommands: true,
			}),
			kong.Vars{
				"version": version.GetVersionString(),
			},
		)
		if err := ctx.Run(); err != nil {
			return
		}
	}
	argss := [][]string{
		{"co", "--", "a.txt", "b.txt"},
		{"co", "master", "--", "a.txt", "b.txt"},
		{"co", ".", "--", "a.txt", "b.txt"},
		{"co", ".", "--", "a.txt", "b.txt", "--"},
	}
	for _, args := range argss {
		parseArgs(args)
	}
}

func TestDiff(t *testing.T) {
	parseArgs := func(args []string) {
		var app App
		ctx := kong.ParseArgs(&app, args,
			kong.Name("zeta"),
			kong.Description(tr.W("HugeSCM - A next generation cloud-based version control system")),
			kong.UsageOnError(),
			kong.ConfigureHelp(kong.HelpOptions{
				Compact:             true,
				NoExpandSubcommands: true,
			}),
			kong.Vars{
				"version": version.GetVersionString(),
			},
		)
		if err := ctx.Run(); err != nil {
			return
		}
	}
	argss := [][]string{
		{"diff", "--", "a.txt", "b.txt"},
		{"diff", "master", "--", "a.txt", "b.txt"},
	}
	for _, args := range argss {
		parseArgs(args)
	}
}
