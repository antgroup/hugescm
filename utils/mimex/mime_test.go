package mimex

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/antgroup/hugescm/modules/chardet"
	"github.com/antgroup/hugescm/modules/mime"
)

func isBinaryPayload(name string, payload []byte) bool {
	result := mime.DetectAny(payload)
	fmt.Fprintf(os.Stderr, "%s mime: %v\n", name, result)
	for p := result; p != nil; p = p.Parent() {
		if p.Is("text/plain") {
			return false
		}
	}
	return true
}

func TestDetectMIME(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	fn := func(name string) {
		bytesO, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			fmt.Fprintf(os.Stderr, "read origin error: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "is binary: %v\n", isBinaryPayload(name, bytesO))
	}
	dirs, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read dir error: %v\n", err)
		return
	}
	for _, d := range dirs {
		if d.IsDir() {
			continue
		}
		fn(d.Name())
	}
}

func textCharset(s string) string {
	if _, charset, ok := strings.Cut(s, ";"); ok {
		return strings.TrimPrefix(strings.TrimSpace(charset), "charset=")
	}
	return "UTF-8"
}

func resolveCharset(payload []byte) string {
	result := mime.DetectAny(payload)
	for p := result; p != nil; p = p.Parent() {
		if p.Is("text/plain") {
			return textCharset(p.String())
		}
	}
	return "binary"
}

func TestDetectCharset(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	fn := func(name string) {
		bytesO, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			fmt.Fprintf(os.Stderr, "read origin error: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "%s charset: %v\n", name, resolveCharset(bytesO))
	}
	dirs, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read dir error: %v\n", err)
		return
	}
	for _, d := range dirs {
		if d.IsDir() {
			continue
		}
		fn(d.Name())
	}
}

func TestChardet(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	d := chardet.NewTextDetector()
	fn := func(name string) {
		bytesO, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			fmt.Fprintf(os.Stderr, "read origin error: %v\n", err)
			return
		}
		result, err := d.DetectBest(bytesO)
		if err != nil {
			fmt.Fprintf(os.Stderr, "detect %s error: %v\n", name, err)
			return
		}
		fmt.Fprintf(os.Stderr, "%s: %s\n", name, result.Charset)
	}
	dirs, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read dir error: %v\n", err)
		return
	}
	for _, d := range dirs {
		if d.IsDir() {
			continue
		}
		fn(d.Name())
	}
}
