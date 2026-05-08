// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"sort"
)

var (
	ErrNotSafeTensors    = errors.New("not a safetensors file")
	ErrInvalidHeaderSize = errors.New("invalid safetensors header size")
)

// SafeTensorsHeader represents the header of a SafeTensors file
type SafeTensorsHeader struct {
	Tensors  map[string]TensorInfo `json:"-"` // Tensor information (dynamically parsed)
	Metadata map[string]string     `json:"__metadata__,omitempty"`
}

// TensorInfo represents tensor metadata
type TensorInfo struct {
	Dtype  string  `json:"dtype"`
	Shape  []int64 `json:"shape"`
	Offset []int64 `json:"data_offsets"` // [start, end)
}

// SafeTensorsParser parses SafeTensors files
type SafeTensorsParser struct {
	headerSize int64
	tensors    []TensorMeta
}

// TensorMeta represents tensor metadata for chunking
type TensorMeta struct {
	Name   string
	Dtype  string
	Shape  []int64
	Offset int64 // Start offset in file
	Size   int64 // Tensor size in bytes
}

// ParseSafeTensors parses the header of a SafeTensors file
func ParseSafeTensors(reader io.ReadSeeker) (*SafeTensorsParser, error) {
	// Read Header Size (first 8 bytes)
	var headerSize uint64
	if err := binary.Read(reader, binary.LittleEndian, &headerSize); err != nil {
		return nil, err
	}

	if headerSize == 0 || headerSize > 100<<20 { // Header max 100MB
		return nil, ErrInvalidHeaderSize
	}

	// Read Header JSON
	headerBytes := make([]byte, headerSize)
	if _, err := io.ReadFull(reader, headerBytes); err != nil {
		return nil, err
	}

	// Parse JSON (dynamic parsing to avoid struct limitations)
	var rawHeader map[string]any
	if err := json.Unmarshal(headerBytes, &rawHeader); err != nil {
		return nil, err
	}

	parser := &SafeTensorsParser{
		headerSize: int64(8 + headerSize),
		tensors:    make([]TensorMeta, 0, len(rawHeader)-1), // Exclude __metadata__
	}

	// Extract tensor metadata
	for name, value := range rawHeader {
		if name == "__metadata__" {
			continue // Skip metadata field
		}

		// Parse tensor information
		if tensor, ok := parseTensorMetadata(name, value, parser.headerSize); ok {
			parser.tensors = append(parser.tensors, tensor)
		}
	}

	// Sort by offset
	sort.Slice(parser.tensors, func(i, j int) bool {
		return parser.tensors[i].Offset < parser.tensors[j].Offset
	})

	return parser, nil
}

// GetChunks returns tensor-level chunks
func (p *SafeTensorsParser) GetChunks() []chunk {
	chunks := make([]chunk, len(p.tensors))
	for i, tensor := range p.tensors {
		chunks[i] = chunk{
			offset: tensor.Offset,
			size:   tensor.Size,
		}
	}
	return chunks
}

// GetTensorMetadata returns tensor metadata
func (p *SafeTensorsParser) GetTensorMetadata() []TensorMeta {
	return p.tensors
}

// parseTensorMetadata parses a single tensor's metadata from raw header
func parseTensorMetadata(name string, value any, headerSize int64) (TensorMeta, bool) {
	tensorMap, ok := value.(map[string]any)
	if !ok {
		return TensorMeta{}, false
	}

	dtype, _ := tensorMap["dtype"].(string)
	shape := parseShape(tensorMap["shape"])
	offsets := parseDataOffsets(tensorMap["data_offsets"])

	if len(offsets) != 2 {
		return TensorMeta{}, false
	}

	start := offsets[0]
	end := offsets[1]

	// Boundary check: ensure offsets are non-negative and start < end
	if start < 0 || end < 0 || start >= end {
		return TensorMeta{}, false
	}

	// Boundary check: ensure size is reasonable (max 100GB per tensor)
	tensorSize := end - start
	if tensorSize > 100<<30 {
		return TensorMeta{}, false
	}

	return TensorMeta{
		Name:   name,
		Dtype:  dtype,
		Shape:  shape,
		Offset: headerSize + start,
		Size:   tensorSize,
	}, true
}

// parseShape parses the shape array from tensor metadata
func parseShape(shapeValue any) []int64 {
	shapeInterface, ok := shapeValue.([]any)
	if !ok {
		return nil
	}

	shape := make([]int64, len(shapeInterface))
	for i, s := range shapeInterface {
		if val, ok := s.(float64); ok {
			shape[i] = int64(val)
		}
	}
	return shape
}

// parseDataOffsets parses the data_offsets array from tensor metadata
func parseDataOffsets(offsetsValue any) []int64 {
	offsetsInterface, ok := offsetsValue.([]any)
	if !ok {
		return nil
	}

	offsets := make([]int64, len(offsetsInterface))
	for i, o := range offsetsInterface {
		if val, ok := o.(float64); ok {
			offsets[i] = int64(val)
		}
	}
	return offsets
}
