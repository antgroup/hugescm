package gitobj

import (
	"testing"
)

func TestUnexpectedObjectTypeErrFormatting(t *testing.T) {
	err := &UnexpectedObjectType{
		Got: TreeObjectType, Wanted: BlobObjectType,
	}

	expected := "git/object: unexpected object type, got: \"tree\", wanted: \"blob\""
	if expected != err.Error() {
		t.Errorf("Expected %v, got %v", expected, err.Error())
	}
}
