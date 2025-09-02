package command

type Recast struct {
	Revision string
	Depth    int
	To       string
}

func (c *Recast) Run(g *Globals) error {

	return nil
}
