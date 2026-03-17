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

	// 构造 SafeTensors Header
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

	// 构造完整的 SafeTensors 文件
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

	parser, err := ParseSafeTensors(reader)
	if err != nil {
		t.Fatalf("ParseSafeTensors failed: %v", err)
	}

	// 验证张量数量
	if len(parser.tensors) != 2 {
		t.Errorf("expected 2 tensors, got %d", len(parser.tensors))
	}

	// 验证第一个张量
	if len(parser.tensors) > 0 {
		tensor1 := parser.tensors[0]
		if tensor1.Name != "tensor1" {
			t.Errorf("expected tensor name 'tensor1', got '%s'", tensor1.Name)
		}
		if tensor1.Size != 800 {
			t.Errorf("expected tensor size 800, got %d", tensor1.Size)
		}
	}

	// 验证第二个张量
	if len(parser.tensors) > 1 {
		tensor2 := parser.tensors[1]
		if tensor2.Name != "tensor2" {
			t.Errorf("expected tensor name 'tensor2', got '%s'", tensor2.Name)
		}
		if tensor2.Size != 100 {
			t.Errorf("expected tensor size 100, got %d", tensor2.Size)
		}
	}
}

func TestSafeTensorsGetChunks(t *testing.T) {
	data := createTestSafeTensors(t)
	reader := bytes.NewReader(data)

	parser, err := ParseSafeTensors(reader)
	if err != nil {
		t.Fatalf("ParseSafeTensors failed: %v", err)
	}

	chunks := parser.GetChunks()
	if len(chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(chunks))
	}

	// 验证分片连续性
	if len(chunks) == 2 {
		if chunks[0].size != 800 {
			t.Errorf("expected chunk size 800, got %d", chunks[0].size)
		}
		if chunks[1].size != 100 {
			t.Errorf("expected chunk size 100, got %d", chunks[1].size)
		}
	}
}

func TestCDCChunker(t *testing.T) {
	// Test CDC chunking with realistic parameters
	data := make([]byte, 10<<20) // 10MB
	for i := range data {
		data[i] = byte(i % 256)
	}

	chunker := NewCDCChunker(4 << 20) // 4MB target (default)
	chunks, err := chunker.Chunk(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// Verify chunks cover the entire file
	var totalSize int64
	for _, c := range chunks {
		totalSize += c.size
	}

	if totalSize != int64(len(data)) {
		t.Errorf("chunks total size %d != data size %d", totalSize, len(data))
	}

	// Verify chunk sizes are within reasonable range
	// minSize = target/4 = 1MB
	// maxSize = target*8 = 32MB
	for i, c := range chunks {
		if c.size < 1<<20 {
			t.Errorf("chunk %d size %d is too small (< 1MB)", i, c.size)
		}
		if c.size > 32<<20 {
			t.Errorf("chunk %d size %d is too large (> 32MB)", i, c.size)
		}
	}

	// Print chunk distribution for debugging
	t.Logf("File size: %d bytes, Chunks: %d", len(data), len(chunks))
	for i, c := range chunks {
		if i < 5 || i >= len(chunks)-2 { // Show first 5 and last 2
			t.Logf("  Chunk %d: offset=%d size=%d", i, c.offset, c.size)
		}
	}
}
