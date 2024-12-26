//go:build appengine || (!linux && !freebsd && !darwin && !dragonfly && !netbsd && !openbsd)
// +build appengine !linux,!freebsd,!darwin,!dragonfly,!netbsd,!openbsd

package kong

import "io"

func guessWidth(_ io.Writer) int {
	return 80
}
