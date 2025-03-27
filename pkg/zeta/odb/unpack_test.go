package odb

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestMetadataUnpack(t *testing.T) {
	tempDir, err := os.MkdirTemp(os.TempDir(), "odb-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create temp dir: %v", err)
		return
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()
	odb, err := NewODB(tempDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create odb dir: %v", err)
		return
	}
	defer odb.Close() // nolint
	if err := odb.MetadataUnpack(strings.NewReader(""), true); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v", err)
		return
	}
}
