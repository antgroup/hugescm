package pack

import (
	"bytes"
	"testing"
)

func TestSetOpenOpensAPackedObject(t *testing.T) {
	const sha = "decafdecafdecafdecafdecafdecafdecafdecaf"
	const data = "Hello, world!\n"
	compressed, _ := compress(data)

	set := NewSetPacks(&Packfile{
		idx: IndexWith(map[string]uint32{
			sha: 0,
		}),
		r: bytes.NewReader(append([]byte{0x3e}, compressed...)),
	})

	o, err := set.Object(DecodeHex(t, sha))

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if TypeBlob != o.Type() {
		t.Errorf("Expected %v, got %v", TypeBlob, o.Type())
	}

	unpacked, err := o.Unpack()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if !bytes.Equal([]byte(data), unpacked) {
		t.Errorf("Expected %v, got %v", []byte(data), unpacked)
	}
}

func TestSetOpenOpensPackedObjectsInPackOrder(t *testing.T) {
	p1 := &Packfile{
		Objects: 1,

		idx: IndexWith(map[string]uint32{
			"aa00000000000000000000000000000000000000": 1,
		}),
		r: bytes.NewReader(nil),
	}
	p2 := &Packfile{
		Objects: 2,

		idx: IndexWith(map[string]uint32{
			"aa11111111111111111111111111111111111111": 1,
			"aa22222222222222222222222222222222222222": 2,
		}),
		r: bytes.NewReader(nil),
	}
	p3 := &Packfile{
		Objects: 3,

		idx: IndexWith(map[string]uint32{
			"aa33333333333333333333333333333333333333": 3,
			"aa44444444444444444444444444444444444444": 4,
			"aa55555555555555555555555555555555555555": 5,
		}),
		r: bytes.NewReader(nil),
	}

	set := NewSetPacks(p1, p2, p3)

	var visited []*Packfile

	_, _ = set.each(
		DecodeHex(t, "aa55555555555555555555555555555555555555"),
		func(p *Packfile) (*Object, error) {
			visited = append(visited, p)
			return nil, errNotFound
		},
	)

	if len(visited) != 3 {
		t.Fatalf("Expected len %v, got %v", 3, len(visited))
	}
	if visited[0].Objects != 3 {
		t.Errorf("Expected %v, got %v", visited[0].Objects, 3)
	}
	if visited[1].Objects != 2 {
		t.Errorf("Expected %v, got %v", visited[1].Objects, 2)
	}
	if visited[2].Objects != 1 {
		t.Errorf("Expected %v, got %v", visited[2].Objects, 1)
	}
}
