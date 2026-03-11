package pack

import (
	"testing"
)

func TestUnsupportedVersionErr(t *testing.T) {
	u := &UnsupportedVersionErr{Got: 3}

	if u.Error() != "git/object/pack:: unsupported version: 3" {
		t.Errorf("Expected 'git/object/pack:: unsupported version: 3', got %v", u.Error())
	}
}
