//go:build windows || darwin

package wildmatch

func init() {
	SystemCase = CaseFold
}
