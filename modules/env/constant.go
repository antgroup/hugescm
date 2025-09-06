package env

import (
	"os"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/strengthen"
)

type K string

// VALUE
const (
	ZETA_TERMINAL_PROMPT  K      = "ZETA_TERMINAL_PROMPT"
	ZETA_NO_SSH_AUTH_SOCK K      = "ZETA_NO_SSH_AUTH_SOCK"
	StandardSeparator     string = ";"
)

func (k K) With(s string) string {
	return string(k) + "=" + s
}

func (k K) WithBool(b bool) string {
	if b {
		return string(k) + "=true"
	}
	return string(k) + "=false"
}

func (k K) WithInt(i int64) string {
	return string(k) + "=" + strconv.FormatInt(i, 10)
}

func (k K) WithPaths(sv []string) string {
	return string(k) + "=" + strings.Join(sv, string(os.PathListSeparator))
}

func (k K) Withs(sv []string) string {
	return string(k) + "=" + strings.Join(sv, StandardSeparator)
}

func (k K) Find() string {
	return os.Getenv(string(k))
}

// find envkey Strings to array
func (k K) Strings() []string {
	s := os.Getenv(string(k))
	return strings.Split(s, StandardSeparator)
}

// find envkey split to array
func (k K) StrSplit(sep string) []string {
	s := os.Getenv(string(k))
	return strings.Split(s, sep)
}

// SimpleAtob Obtain the boolean variable from the environment variable, if it does not exist, return the default value
func (k K) SimpleAtob(dv bool) bool {
	s, ok := os.LookupEnv(string(k))
	if !ok {
		return dv
	}
	return strengthen.SimpleAtob(s, dv)
}

func (k K) SimpleAtoi(dv int64) int64 {
	s, ok := os.LookupEnv(string(k))
	if !ok {
		return dv
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	return dv
}
