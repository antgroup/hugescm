package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestDecode(t *testing.T) {
	var cc Config
	_, filename, _, _ := runtime.Caller(0)
	file := filepath.Join(filepath.Dir(filename), "config_test.toml")
	fd, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open error: %v\n", err)
		return
	}
	defer fd.Close()
	meta, err := toml.NewDecoder(fd).Decode(&cc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open error: %v\n", err)
		return
	}
	for _, k := range meta.Keys() {
		ks := k.String()
		fmt.Fprintf(os.Stderr, "%v %s\n", ks, meta.Type(ks))
	}
}

func TestDecode2(t *testing.T) {
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
	d := &DisplayOptions{Writer: os.Stderr, Z: false}
	for k, s := range sections {
		if s == nil {
			continue
		}
		if err := s.displayTo(d, k); err != nil {
			return
		}
	}
}

func TestDecodeZ(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	p := filepath.Join(filepath.Dir(filename), "config_test.toml")
	sections := make(Sections)
	if _, err := toml.DecodeFile(p, &sections); err != nil {
		fmt.Fprintf(os.Stderr, "open error: %v\n", err)
		return
	}
	d := &DisplayOptions{Writer: os.Stderr, Z: true}
	for k, s := range sections {
		if s == nil {
			continue
		}
		if err := s.displayTo(d, k); err != nil {
			return
		}
	}
}

func TestFilter(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	p := filepath.Join(filepath.Dir(filename), "config_test.toml")
	sections := make(Sections)
	if _, err := toml.DecodeFile(p, &sections); err != nil {
		fmt.Fprintf(os.Stderr, "open error: %v\n", err)
		return
	}
	vals, err := sections.filterAll("core.sparse-checkout")
	if err != nil {
		fmt.Fprintf(os.Stderr, "filter all: %v\n", err)
		return
	}
	for _, v := range vals {
		fmt.Fprintf(os.Stderr, "values: %s\n", v)
	}
}

func TestLoad(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	p := filepath.Join(filepath.Dir(filename), "config_test_bad.toml")
	var rc Config
	if _, err := toml.DecodeFile(p, &rc); err != nil {
		fmt.Fprintf(os.Stderr, "decode error: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "%v\n", rc)
}
