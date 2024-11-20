// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package serve

import (
	"embed"
	"errors"
	"net/http"
	"path"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

//go:embed languages
var langFS embed.FS

type LanguageDict map[string]any

func (d LanguageDict) translateTo(text string) string {
	if v, ok := d[text]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return text
}

var (
	languagesDicts     = make(map[string]LanguageDict)
	languagesSupported = []string{"en-US"}
	languageMatcher    language.Matcher
)

func parseOneDict(name string, p string) error {
	dict := make(LanguageDict)
	fd, err := langFS.Open(p)
	if err != nil {
		return err
	}
	defer fd.Close()
	if _, err := toml.NewDecoder(fd).Decode(&dict); err != nil {
		return err
	}
	languagesDicts[name] = dict
	languagesSupported = append(languagesSupported, name)
	return nil
}

func registerLanguages() error {
	dirs, err := langFS.ReadDir("languages")
	if err != nil {
		return nil
	}
	for _, d := range dirs {
		if d.IsDir() {
			continue
		}
		name, ok := strings.CutSuffix(d.Name(), ".toml")
		if !ok {
			continue
		}
		if err := parseOneDict(name, path.Join("languages", d.Name())); err != nil {
			logrus.Errorf("load language '%s' error: %v", name, err)
			continue
		}
	}
	return nil
}

func T(lang, text string) string {
	if d, ok := languagesDicts[lang]; ok {
		return d.translateTo(text)
	}
	return text
}

func RegisterLanguageMatcher() error {
	if err := registerLanguages(); err != nil {
		return err
	}
	tags := []language.Tag{}
	for _, lang := range languagesSupported {
		if tag, err := language.Parse(lang); err == nil {
			tags = append(tags, tag)
		}
	}
	if len(tags) == 0 {
		return errors.New("empty languages tags")
	}
	languageMatcher = language.NewMatcher(tags)
	return nil
}

func Language(r *http.Request) string {
	if languageMatcher == nil {
		return "en-US"
	}
	lang, _ := r.Cookie("lang")
	accept := r.Header.Get("Accept-Language")
	tag, _ := language.MatchStrings(languageMatcher, lang.String(), accept)
	return tag.String()
}

func W(r *http.Request, message string) string {
	if languageMatcher == nil {
		return message
	}
	lang, _ := r.Cookie("lang")
	accept := r.Header.Get("Accept-Language")
	tag, _ := language.MatchStrings(languageMatcher, lang.String(), accept)
	return T(tag.String(), message)
}
