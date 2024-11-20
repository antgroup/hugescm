//go:build ignore
// +build ignore

package locale

var detectors = []detector{
	detectViaEnvLanguage,
	detectViaEnvLc,
}
