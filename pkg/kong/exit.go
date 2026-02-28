package kong

import "errors"

const (
	exitOk    = 0
	exitNotOk = 1

	// Semantic exit codes from https://github.com/square/exit?tab=readme-ov-file#about
	exitUsageError = 80
)

// ExitCoder is an interface that may be implemented by an error value to
// provide an integer exit code. The method ExitCode should return an integer
// that is intended to be used as the exit code for the application.
type ExitCoder interface {
	error
	ExitCode() int
}

// exitCodeFromError returns the exit code for the given error.
// If err implements the exitCoder interface, the ExitCode method is called.
// Otherwise, exitCodeFromError returns 0 if err is nil, and 1 if it is not.
func exitCodeFromError(err error) int {
	if e, ok := errors.AsType[ExitCoder](err); ok {
		return e.ExitCode()
	} else if err == nil {
		return exitOk
	}

	return exitNotOk
}
