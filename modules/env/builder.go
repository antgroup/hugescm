package env

type Builder interface {
	Environ() []string
}

type builder struct {
}

func (b *builder) Environ() []string {
	return Environ()
}

func NewBuilder() Builder {
	return &builder{}
}
