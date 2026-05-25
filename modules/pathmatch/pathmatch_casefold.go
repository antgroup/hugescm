//go:build windows || darwin

package pathmatch

func init() {
	SystemCase = CaseFold
}
