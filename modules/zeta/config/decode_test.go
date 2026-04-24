package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDecode(t *testing.T) {
	var cc Config
	_, filename, _, _ := runtime.Caller(0)
	file := filepath.Join(filepath.Dir(filename), "config_test.toml")
	if err := LoadConfigFile(file, &cc); err != nil {
		fmt.Fprintf(os.Stderr, "decode error: %v\n", err)
		return
	}
}

func TestDecode2(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	file := filepath.Join(filepath.Dir(filename), "config_test.toml")
	doc, err := LoadDocumentFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load error: %v\n", err)
		return
	}
	d := &DisplayOptions{Writer: os.Stderr, Z: false}
	for k, s := range doc {
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
	doc, err := LoadDocumentFile(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load error: %v\n", err)
		return
	}
	d := &DisplayOptions{Writer: os.Stderr, Z: true}
	for k, s := range doc {
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
	doc, err := LoadDocumentFile(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load error: %v\n", err)
		return
	}
	vals, err := doc.GetAll("core.sparse-checkout")
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
	if err := LoadConfigFile(p, &rc); err != nil {
		fmt.Fprintf(os.Stderr, "decode error: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "%v\n", rc)
}
