package command

type Co struct {
	From        string   `arg:"" name:"from" help:"Original repository remote URL" type:"string"`
	Destination string   `arg:"" optional:"" name:"destination" help:"Destination for the new repository" type:"path"`
	Branch      string   `name:"branch" short:"b" help:"Instead of pointing the newly created HEAD to the branch pointed to by the cloned repository’s HEAD, point to <name> branch instead"`
	Commit      string   `name:"commit" short:"c" help:"Instead of pointing the newly created HEAD to the branch pointed to by the cloned repository’s HEAD, point to <name> commit instead"`
	Sparse      []string `name:"sparse" short:"s" help:"A subset of repository files, all files are checked out by default" type:"string"`
	Depth       int      `name:"depth" short:"d" default:"10" help:"Create a shallow clone with a history truncated to the specified number of commits"`
	Limit       int64    `name:"limit" short:"L" help:"Omits blobs larger than n bytes or units. n may be zero. supported units: KB,MB,GB,K,M,G" default:"-1" type:"size"`
	Recursive   bool     `name:"recursive" short:"r" help:"After the clone is created, initialize and clone submodules within based on the provided pathspec"`
}

func (c *Co) Run(g *Globals) error {

	return nil
}
