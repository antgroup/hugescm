package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestEncode(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	file := filepath.Join(filepath.Dir(filename), "config_test.toml")
	fd, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open error: %v\n", err)
		return
	}
	defer fd.Close()
	mc := make(map[string]any)
	_, err = toml.NewDecoder(fd).Decode(&mc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open error: %v\n", err)
		return
	}
	mc["user"] = &User{
		Email: "zeta@example.io",
		Name:  "bob",
	}
	enc := toml.NewEncoder(os.Stdout)
	enc.Indent = ""
	if err := enc.Encode(mc); err != nil {
		fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
		return
	}
}

func TestUpdateConfig(t *testing.T) {
	values := map[string]any{
		"core.sharingRoot": "/tmp/sharingRoot",
		"user.email":       "zeta@example.io",
		"user.name":        "bob",
	}
	_ = UpdateLocal("/tmp/testconfig/.zeta", &UpdateOptions{Values: values})

	values["user.name"] = "Staff"
	_ = UpdateLocal("/tmp/testconfig/.zeta", &UpdateOptions{Values: values})
}

func TestEncodeInt(t *testing.T) {
	s := &Core{}
	if err := toml.NewEncoder(os.Stderr).Encode(s); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}
}

func TestUpdateKey(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	file := filepath.Join(filepath.Dir(filename), "config_test.toml")
	fd, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open error: %v\n", err)
		return
	}
	defer fd.Close()
	sections := make(Sections)
	if _, err = toml.NewDecoder(fd).Decode(&sections); err != nil {
		fmt.Fprintf(os.Stderr, "open error: %v\n", err)
		return
	}
	_, _ = sections.updateKey("core.sparse-checkout", "dev/jack", true)
	enc := toml.NewEncoder(os.Stderr)
	enc.Indent = ""
	if err := enc.Encode(sections); err != nil {
		fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
	}
}

func TestUpdateNot(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	file := filepath.Join(filepath.Dir(filename), "config_test.toml")
	fd, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open error: %v\n", err)
		return
	}
	defer fd.Close()
	sections := make(Sections)
	if _, err = toml.NewDecoder(fd).Decode(&sections); err != nil {
		fmt.Fprintf(os.Stderr, "open error: %v\n", err)
		return
	}
	_, _ = sections.updateKey("core.sparse-checkout", 10086, true)
	enc := toml.NewEncoder(os.Stderr)
	enc.Indent = ""
	if err := enc.Encode(sections); err != nil {
		fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
	}
}

func TestUpdateNot2(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	file := filepath.Join(filepath.Dir(filename), "config_test.toml")
	fd, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open error: %v\n", err)
		return
	}
	defer fd.Close()
	sections := make(Sections)
	if _, err = toml.NewDecoder(fd).Decode(&sections); err != nil {
		fmt.Fprintf(os.Stderr, "open error: %v\n", err)
		return
	}
	_, _ = sections.updateKey("core.namespace", 10086, true)
	enc := toml.NewEncoder(os.Stderr)
	enc.Indent = ""
	if err := enc.Encode(sections); err != nil {
		fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
	}
}
