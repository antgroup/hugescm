// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

// createTestSafeTensors 创建测试用的 SafeTensors 文件
func createTestSafeTensors(t *testing.T) []byte {
	t.Helper()

	header := map[string]any{
		"tensor1": map[string]any{
			"dtype":        "F32",
			"shape":        []int64{10, 20},
			"data_offsets": []int64{0, 800}, // 10*20*4 = 800 字节
		},
		"tensor2": map[string]any{
			"dtype":        "F16",
			"shape":        []int64{5, 10},
			"data_offsets": []int64{800, 900}, // 5*10*2 = 100 字节
		},
		"__metadata__": map[string]any{
			"model": "test-model",
		},
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}

	buf := &bytes.Buffer{}

	// 1. 写入 Header Size (8 字节)
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(headerBytes))); err != nil {
		t.Fatalf("write header size: %v", err)
	}
	// 2. 写入 Header JSON
	if _, err := buf.Write(headerBytes); err != nil {
		t.Fatalf("write header: %v", err)
	}
	// 3. 写入张量数据(简化为填充零)
	tensorData := make([]byte, 900)
	if _, err := buf.Write(tensorData); err != nil {
		t.Fatalf("write tensor data: %v", err)
	}
	return buf.Bytes()
}

func TestParseSafeTensors(t *testing.T) {
	data := createTestSafeTensors(t)
	reader := bytes.NewReader(data)

	file, err := ParseSafeTensors(reader, -1)
	if err != nil {
		t.Fatalf("ParseSafeTensors failed: %v", err)
	}

	if len(file.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(file.Entries))
	}

	if len(file.Entries) > 0 {
		e := file.Entries[0]
		if e.Name != "tensor1" {
			t.Errorf("expected entry name 'tensor1', got '%s'", e.Name)
		}
		if e.Size != 800 {
			t.Errorf("expected entry size 800, got %d", e.Size)
		}
	}

	if len(file.Entries) > 1 {
		e := file.Entries[1]
		if e.Name != "tensor2" {
			t.Errorf("expected entry name 'tensor2', got '%s'", e.Name)
		}
		if e.Size != 100 {
			t.Errorf("expected entry size 100, got %d", e.Size)
		}
	}
}

func TestSafeTensorsFileChunks(t *testing.T) {
	data := createTestSafeTensors(t)
	reader := bytes.NewReader(data)

	file, err := ParseSafeTensors(reader, -1)
	if err != nil {
		t.Fatalf("ParseSafeTensors failed: %v", err)
	}

	spans := file.Chunks()
	if len(spans) != 2 {
		t.Errorf("expected 2 spans, got %d", len(spans))
	}

	if len(spans) == 2 {
		if spans[0].Size != 800 {
			t.Errorf("expected span size 800, got %d", spans[0].Size)
		}
		if spans[1].Size != 100 {
			t.Errorf("expected span size 100, got %d", spans[1].Size)
		}
	}
}

func TestParseSafeTensorsRejectsOutOfBoundsEntry(t *testing.T) {
	data := createTestSafeTensors(t)
	// Compute payload size for cross-file validation.
	headerSize := binary.LittleEndian.Uint64(data[:8])
	dataSize := int64(len(data)) - 8 - int64(headerSize)
	// Sanity: with the real dataSize, parse should succeed.
	if _, err := ParseSafeTensors(bytes.NewReader(data), dataSize); err != nil {
		t.Fatalf("expected ok with real dataSize, got %v", err)
	}
	// Shrink dataSize to provoke a bounds violation.
	if _, err := ParseSafeTensors(bytes.NewReader(data), dataSize-1); err == nil {
		t.Errorf("expected error when dataSize is too small, got nil")
	}
}

func TestParseSafeTensorsLargeOffsetsKeepPrecision(t *testing.T) {
	// 5 GiB+ offset — well beyond float64's 53-bit mantissa for integers.
	const start = int64(1) << 53
	const end = start + 1024
	header := map[string]any{
		"big": map[string]any{
			"dtype":        "U8",
			"shape":        []int64{1024},
			"data_offsets": []int64{start, end},
		},
	}
	hb, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(hb))); err != nil {
		t.Fatal(err)
	}
	buf.Write(hb)

	file, err := ParseSafeTensors(bytes.NewReader(buf.Bytes()), -1)
	if err != nil {
		t.Fatalf("ParseSafeTensors: %v", err)
	}
	if len(file.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(file.Entries))
	}
	got := file.Entries[0]
	if got.Size != end-start {
		t.Errorf("size: got %d want %d", got.Size, end-start)
	}
	wantAbs := file.HeaderSize + start
	if got.Offset != wantAbs {
		t.Errorf("absolute offset: got %d want %d", got.Offset, wantAbs)
	}
}
