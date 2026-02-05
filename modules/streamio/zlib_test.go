package streamio

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestZlibEncode(t *testing.T) {
	content := `Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`

	for range 100 {
		var buf bytes.Buffer
		z := GetZlibWriter(&buf)
		if _, err := io.Copy(z, strings.NewReader(content)); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		PutZlibWriter(z)
	}
}

func TestZlibDecode(t *testing.T) {
	content := `Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`
	var buf bytes.Buffer
	z := GetZlibWriter(&buf)
	_, _ = io.Copy(z, strings.NewReader(content))
	PutZlibWriter(z)
	for i := range 100 {
		z, err := GetZlibReader(bytes.NewReader(buf.Bytes()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "decode error: %v\n", err)
			PutZlibReader(z)
			continue
		}
		_, _ = io.Copy(io.Discard, z.Reader)
		fmt.Fprintf(os.Stderr, "%d\n", i)
		PutZlibReader(z)
	}
}

func TestZlibEncodeDecode(t *testing.T) {
	testCases := []string{
		"",
		"hello world",
		"Hello, 世界!",
		strings.Repeat("a", 1000),
		strings.Repeat("hello ", 1000),
	}

	for _, content := range testCases {
		t.Run(fmt.Sprintf("len=%d", len(content)), func(t *testing.T) {
			// Encode
			var compressed bytes.Buffer
			writer := GetZlibWriter(&compressed)
			_, err := io.Copy(writer, strings.NewReader(content))
			if err != nil {
				t.Fatalf("encode error: %v", err)
			}
			err = writer.Close()
			if err != nil {
				t.Fatalf("writer close error: %v", err)
			}
			PutZlibWriter(writer)

			// Decode
			reader, err := GetZlibReader(bytes.NewReader(compressed.Bytes()))
			if err != nil {
				t.Fatalf("get reader error: %v", err)
			}
			var decompressed bytes.Buffer
			_, err = io.Copy(&decompressed, reader.Reader)
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			PutZlibReader(reader)

			// Verify
			if decompressed.String() != content {
				t.Errorf("decompressed content mismatch:\ngot: %q\nwant: %q",
					decompressed.String(), content)
			}
		})
	}
}

func TestZlibInvalidData(t *testing.T) {
	invalidData := []byte{0x00, 0x01, 0x02, 0x03}

	_, err := GetZlibReader(bytes.NewReader(invalidData))
	if err == nil {
		t.Error("expected error for invalid zlib data, got nil")
	}
}

func TestZlibConcurrent(t *testing.T) {
	content := strings.Repeat("concurrent test data ", 1000)

	var compressed bytes.Buffer
	writer := GetZlibWriter(&compressed)
	_, _ = io.Copy(writer, strings.NewReader(content))
	_ = writer.Close()
	PutZlibWriter(writer)

	done := make(chan bool, 10)
	for range 10 {
		go func() {
			for range 100 {
				reader, err := GetZlibReader(bytes.NewReader(compressed.Bytes()))
				if err != nil {
					fmt.Fprintf(os.Stderr, "concurrent decode error: %v\n", err)
					continue
				}
				var decompressed bytes.Buffer
				_, _ = io.Copy(&decompressed, reader.Reader)
				PutZlibReader(reader)

				if decompressed.String() != content {
					fmt.Fprintf(os.Stderr, "concurrent data mismatch\n")
				}
			}
			done <- true
		}()
	}

	for range 10 {
		<-done
	}
}

func TestZlibEmptyInput(t *testing.T) {
	// Test with empty input
	var buf bytes.Buffer
	writer := GetZlibWriter(&buf)
	_, err := writer.Write([]byte{})
	if err != nil {
		t.Fatalf("write empty error: %v", err)
	}
	err = writer.Close()
	if err != nil {
		t.Fatalf("close error: %v", err)
	}
	PutZlibWriter(writer)

	// Should be able to decompress
	reader, err := GetZlibReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("get reader error: %v", err)
	}
	var decompressed bytes.Buffer
	_, err = io.Copy(&decompressed, reader.Reader)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	PutZlibReader(reader)

	if decompressed.Len() != 0 {
		t.Errorf("expected empty decompressed data, got %d bytes", decompressed.Len())
	}
}

func TestZlibMultipleWrite(t *testing.T) {
	content := "hello world"
	var buf bytes.Buffer

	writer := GetZlibWriter(&buf)
	_, err := writer.Write([]byte(content[:5]))
	if err != nil {
		t.Fatalf("first write error: %v", err)
	}
	_, err = writer.Write([]byte(content[5:]))
	if err != nil {
		t.Fatalf("second write error: %v", err)
	}
	err = writer.Close()
	if err != nil {
		t.Fatalf("close error: %v", err)
	}
	PutZlibWriter(writer)

	// Decompress and verify
	reader, err := GetZlibReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("get reader error: %v", err)
	}
	var decompressed bytes.Buffer
	_, err = io.Copy(&decompressed, reader.Reader)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	PutZlibReader(reader)

	if decompressed.String() != content {
		t.Errorf("decompressed content mismatch:\ngot: %q\nwant: %q",
			decompressed.String(), content)
	}
}

func TestZlibPoolReuse(t *testing.T) {
	content := "test content for pool reuse"

	for i := range 100 {
		// Compress
		var compressed bytes.Buffer
		writer := GetZlibWriter(&compressed)
		_, err := io.Copy(writer, strings.NewReader(content))
		if err != nil {
			t.Fatalf("encode error: %v", err)
		}
		err = writer.Close()
		if err != nil {
			t.Fatalf("writer close error: %v", err)
		}
		PutZlibWriter(writer)

		// Decompress
		reader, err := GetZlibReader(bytes.NewReader(compressed.Bytes()))
		if err != nil {
			t.Fatalf("get reader error: %v", err)
		}
		var decompressed bytes.Buffer
		_, err = io.Copy(&decompressed, reader.Reader)
		if err != nil {
			t.Fatalf("decode error: %v", err)
		}
		PutZlibReader(reader)

		if decompressed.String() != content {
			t.Errorf("iteration %d: decompressed content mismatch", i)
		}
	}
}
