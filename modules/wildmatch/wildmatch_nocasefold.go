//go:build !windows && !darwin
// +build !windows,!darwin

package wildmatch

func init() {
	SystemCase = func(w *Wildmatch) {}
}
