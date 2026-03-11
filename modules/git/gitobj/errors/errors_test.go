package errors

import (
	"encoding/hex"
	"testing"
)

func TestNoSuchObjectTypeErrFormatting(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	oid, err := hex.DecodeString(sha)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	err = NoSuchObject(oid)

	if err.Error() != "git/object: no such object: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Errorf("Expected %v, got %v", "git/object: no such object: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", err.Error())
	}
	if IsNoSuchObject(err) != true {
		t.Errorf("Expected %v, got %v", IsNoSuchObject(err), true)
	}
}

func TestIsNoSuchObjectNilHandling(t *testing.T) {
	if IsNoSuchObject((*noSuchObject)(nil)) != false {
		t.Errorf("Expected %v, got %v", IsNoSuchObject((*noSuchObject)(nil)), false)
	}
	if IsNoSuchObject(nil) != false {
		t.Errorf("Expected %v, got %v", IsNoSuchObject(nil), false)
	}
}
