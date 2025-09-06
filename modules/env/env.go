package env

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func SanitizerEnv(removeKey ...string) []string {
	removeMap := make(map[string]bool)
	for _, k := range removeKey {
		removeMap[k] = true
	}
	originEnv := os.Environ()
	env := make([]string, 0, len(originEnv))
	for _, e := range originEnv {
		k, _, ok := strings.Cut(e, "=")
		if !ok {
			// BAD env
			continue
		}
		if removeMap[k] {
			continue
		}
		env = append(env, e)
	}
	return env
}

// GetBool fetches and parses a boolean typed environment variable
//
// If the variable is empty, returns `fallback` and no error.
// If there is an error, returns `fallback` and the error.
func GetBool(name string, fallback bool) (bool, error) {
	s := os.Getenv(name)
	if s == "" {
		return fallback, nil
	}
	v, err := strconv.ParseBool(s)
	if err != nil {
		return fallback, fmt.Errorf("get bool %s: %w", name, err)
	}
	return v, nil
}

// GetInt fetches and parses an integer typed environment variable
//
// If the variable is empty, returns `fallback` and no error.
// If there is an error, returns `fallback` and the error.
func GetInt(name string, fallback int) (int, error) {
	s := os.Getenv(name)
	if s == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback, fmt.Errorf("get int %s: %w", name, err)
	}
	return v, nil
}

// GetDuration fetches and parses a duration typed environment variable
func GetDuration(name string, fallback time.Duration) (time.Duration, error) {
	s := os.Getenv(name)
	if s == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return fallback, fmt.Errorf("get duration %s: %w", name, err)
	}
	return v, nil
}

// GetString fetches a given name from the environment and falls back to a
// default value if the name is not available. The value is stripped of
// leading and trailing whitespace.
func GetString(name string, fallback string) string {
	value := os.Getenv(name)

	if value == "" {
		return fallback
	}

	return strings.TrimSpace(value)
}
