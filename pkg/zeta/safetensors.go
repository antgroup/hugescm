// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
)

// SafeTensors errors.
var (
	ErrNotSafeTensors    = errors.New("not a safetensors file")
	ErrInvalidHeaderSize = errors.New("invalid safetensors header size")
)

// safeTensorsHeaderMax bounds the JSON header to avoid pathological
// allocations from malformed inputs. 16MiB is comfortably above the largest
// headers seen in practice (a few MiB for very large checkpoints).
const safeTensorsHeaderMax = 16 << 20

// SafeTensorsFile represents a parsed safetensors file header.
//
// Naming follows the safetensors specification: the per-tensor records in
// the JSON header are called "entries"; the optional top-level "__metadata__"
// map is a separate concept and is intentionally not modeled here.
type SafeTensorsFile struct {
	HeaderSize int64         // Size of the 8-byte length prefix + JSON header.
	Entries    []TensorEntry // Tensor entries sorted by absolute file offset.
}

// TensorEntry is a single tensor record extracted from the safetensors header,
// translated into absolute file coordinates.
type TensorEntry struct {
	Name   string
	Dtype  string
	Shape  []int64
	Offset int64 // Absolute start offset in the file (header size + data start).
	Size   int64 // Tensor payload size in bytes.
}

// ParseSafeTensors parses the header of a safetensors stream.
//
// dataSize is the number of payload bytes that follow the header (i.e.
// fileSize - HeaderSize). Pass -1 if unknown to skip the cross-file bounds
// check; callers that have a reliable size should always pass it.
//
// The reader is consumed sequentially; no Seek is required.
func ParseSafeTensors(reader io.Reader, dataSize int64) (*SafeTensorsFile, error) {
	// Read Header Size (first 8 bytes).
	var headerSize uint64
	if err := binary.Read(reader, binary.LittleEndian, &headerSize); err != nil {
		return nil, err
	}

	if headerSize == 0 || headerSize > safeTensorsHeaderMax {
		return nil, ErrInvalidHeaderSize
	}

	// Read Header JSON.
	headerBytes := make([]byte, headerSize)
	if _, err := io.ReadFull(reader, headerBytes); err != nil {
		return nil, err
	}

	// Parse JSON with UseNumber so offsets retain full uint64 precision.
	// (Float64 only carries 53 bits of mantissa, which is not enough for
	// per-tensor offsets in 100GB+ checkpoints.)
	dec := json.NewDecoder(bytes.NewReader(headerBytes))
	dec.UseNumber()
	var rawHeader map[string]any
	if err := dec.Decode(&rawHeader); err != nil {
		return nil, err
	}

	// Capacity is best-effort: __metadata__ may be absent.
	capHint := len(rawHeader)
	if _, ok := rawHeader["__metadata__"]; ok {
		capHint--
	}
	if capHint < 0 {
		capHint = 0
	}

	file := &SafeTensorsFile{
		HeaderSize: int64(8 + headerSize),
		Entries:    make([]TensorEntry, 0, capHint),
	}

	for name, value := range rawHeader {
		if name == "__metadata__" {
			continue
		}
		entry, err := parseEntry(name, value, file.HeaderSize, dataSize)
		if err != nil {
			return nil, err
		}
		if entry == nil {
			continue // entry was unparseable but not fatal (e.g. missing fields)
		}
		file.Entries = append(file.Entries, *entry)
	}

	// Sort by absolute offset so consumers can stream sequentially.
	sort.Slice(file.Entries, func(i, j int) bool {
		return file.Entries[i].Offset < file.Entries[j].Offset
	})

	// Validate that entries cover [0, dataSize) without overlaps. The
	// safetensors specification requires contiguous, non-overlapping data
	// regions.
	if dataSize >= 0 {
		var cursor int64
		for i, e := range file.Entries {
			start := e.Offset - file.HeaderSize
			if start < cursor {
				return nil, fmt.Errorf("safetensors: entry %d (%s) overlaps previous (start=%d, cursor=%d)", i, e.Name, start, cursor)
			}
			// Gaps between tensors are not allowed by the spec, but we tolerate
			// them rather than reject otherwise valid files.
			cursor = start + e.Size
			if cursor > dataSize {
				return nil, fmt.Errorf("safetensors: entry %d (%s) extends past data end (cursor=%d, dataSize=%d)", i, e.Name, cursor, dataSize)
			}
		}
	}

	return file, nil
}

// Chunks returns one Span per tensor entry, in offset order.
//
// These spans align chunk boundaries to tensor boundaries, which is the
// natural unit of change for AI model files.
func (f *SafeTensorsFile) Chunks() []Span {
	out := make([]Span, len(f.Entries))
	for i, e := range f.Entries {
		out[i] = Span{Offset: e.Offset, Size: e.Size}
	}
	return out
}

// parseEntry decodes a single tensor record from the raw JSON header.
//
// Returns (nil, nil) when the record is structurally invalid but should be
// ignored (e.g. unexpected types from a malformed header). Returns an error
// only when the file is clearly corrupted (e.g. bounds violations).
func parseEntry(name string, value any, headerSize, dataSize int64) (*TensorEntry, error) {
	tensorMap, ok := value.(map[string]any)
	if !ok {
		return nil, nil
	}

	dtype, _ := tensorMap["dtype"].(string)
	shape := decodeShape(tensorMap["shape"])
	offsets, err := decodeOffsets(tensorMap["data_offsets"])
	if err != nil {
		return nil, fmt.Errorf("safetensors: entry %q: %w", name, err)
	}

	if len(offsets) != 2 {
		return nil, nil
	}

	start, end := offsets[0], offsets[1]
	if start < 0 || end < 0 || start >= end {
		return nil, fmt.Errorf("safetensors: entry %q: invalid data_offsets [%d, %d)", name, start, end)
	}

	size := end - start
	if size > 100<<30 { // 100GB per tensor
		return nil, fmt.Errorf("safetensors: entry %q: tensor too large (%d bytes)", name, size)
	}

	if dataSize >= 0 && end > dataSize {
		return nil, fmt.Errorf("safetensors: entry %q: extends past data end (end=%d, dataSize=%d)", name, end, dataSize)
	}

	// Overflow guard for absolute offset arithmetic.
	if headerSize > 0 && start > (1<<62-headerSize) {
		return nil, fmt.Errorf("safetensors: entry %q: absolute offset overflow", name)
	}

	return &TensorEntry{
		Name:   name,
		Dtype:  dtype,
		Shape:  shape,
		Offset: headerSize + start,
		Size:   size,
	}, nil
}

// decodeShape converts the JSON shape array to []int64.
//
// Accepts both json.Number (when the decoder uses UseNumber) and float64
// (legacy behavior, for robustness against external callers).
func decodeShape(v any) []int64 {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]int64, len(arr))
	for i, s := range arr {
		switch x := s.(type) {
		case json.Number:
			if n, err := x.Int64(); err == nil {
				out[i] = n
			}
		case float64:
			out[i] = int64(x)
		}
	}
	return out
}

// decodeOffsets converts the JSON data_offsets array to []int64, preserving
// full uint64 precision via json.Number.
func decodeOffsets(v any) ([]int64, error) {
	arr, ok := v.([]any)
	if !ok {
		return nil, nil
	}
	out := make([]int64, len(arr))
	for i, o := range arr {
		switch x := o.(type) {
		case json.Number:
			n, err := x.Int64()
			if err != nil {
				return nil, fmt.Errorf("data_offsets[%d]: %w", i, err)
			}
			out[i] = n
		case float64:
			out[i] = int64(x)
		default:
			return nil, fmt.Errorf("data_offsets[%d]: unexpected type %T", i, o)
		}
	}
	return out, nil
}
