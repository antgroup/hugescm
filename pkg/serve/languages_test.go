package serve

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/text/language"
)

func TestW(t *testing.T) {
	_ = RegisterLanguageMatcher()
	langKey := ParseLangEnv("zh_CN.UTF-8")
	fmt.Fprintf(os.Stderr, Translate(langKey, "branch '%s' not exist"), "dev-99")
}

func TestAcceptLanguages(t *testing.T) {
	accept := "zh-CN"
	_ = RegisterLanguageMatcher()
	tag, _ := language.MatchStrings(languageMatcher, "", accept)
	fmt.Fprintf(os.Stderr, "accept-language: %s\n", tag.String())
}
