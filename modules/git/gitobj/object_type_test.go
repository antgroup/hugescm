package gitobj

import (
	"math"
	"testing"
)

func TestObjectTypeFromString(t *testing.T) {
	for str, typ := range map[string]ObjectType{
		"blob":           BlobObjectType,
		"tree":           TreeObjectType,
		"commit":         CommitObjectType,
		"tag":            TagObjectType,
		"something else": UnknownObjectType,
	} {
		t.Run(str, func(t *testing.T) {
			if typ != ObjectTypeFromString(str) {
				t.Errorf("Expected %v, got %v", typ, ObjectTypeFromString(str))
			}
		})
	}
}

func TestObjectTypeToString(t *testing.T) {
	for typ, str := range map[ObjectType]string{
		BlobObjectType:            "blob",
		TreeObjectType:            "tree",
		CommitObjectType:          "commit",
		TagObjectType:             "tag",
		UnknownObjectType:         "unknown",
		ObjectType(math.MaxUint8): "<unknown>",
	} {
		t.Run(str, func(t *testing.T) {
			if str != typ.String() {
				t.Errorf("Expected %v, got %v", str, typ.String())
			}
		})
	}
}
