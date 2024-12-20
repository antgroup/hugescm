package gitobj

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnexpectedObjectTypeErrFormatting(t *testing.T) {
	err := &UnexpectedObjectType{
		Got: TreeObjectType, Wanted: BlobObjectType,
	}

	assert.Equal(t, "git/object: unexpected object type, got: \"tree\", wanted: \"blob\"", err.Error())
}
