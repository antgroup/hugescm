package trace

type Debuger interface {
	DbgPrint(format string, args ...any)
}
