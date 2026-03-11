package pack

import (
	"bytes"
	"errors"
	"testing"
)

func TestObjectTypeReturnsObjectType(t *testing.T) {
	o := &Object{
		typ: TypeCommit,
	}

	if TypeCommit != o.Type() {
		t.Errorf("Expected %v, got %v", TypeCommit, o.Type())
	}
}

func TestObjectUnpackUnpacksData(t *testing.T) {
	expected := []byte{0x1, 0x2, 0x3, 0x4}

	o := &Object{
		data: &ChainSimple{
			X: expected,
		},
	}

	data, err := o.Unpack()

	if !bytes.Equal(expected, data) {
		t.Errorf("Expected %v, got %v", expected, data)
	}
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
}

func TestObjectUnpackPropogatesErrors(t *testing.T) {
	expected := errors.New("git/object/pack: testing")

	o := &Object{
		data: &ChainSimple{
			Err: expected,
		},
	}

	data, err := o.Unpack()

	if data != nil {
		t.Errorf("Expected nil, got %v", data)
	}
	if expected != err {
		t.Errorf("Expected %v, got %v", expected, err)
	}
}
