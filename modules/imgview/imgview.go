// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package imgview renders raster image blobs as inline-image escape sequences
// for terminals that support the iTerm2 or Kitty graphics protocols.
//
// Detection of which protocol the host terminal supports is owned by
// modules/term; this package is intentionally protocol-aware but
// terminal-agnostic so it stays easy to unit-test by inspecting the byte
// stream it writes.
package imgview

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"

	"github.com/antgroup/hugescm/modules/mime"
	"github.com/antgroup/hugescm/modules/term"
)

// MaxImageBytes caps the size of a single image payload this package will
// render. Anything larger should fall back to a hex dump in the caller.
//
// Both protocols base64-encode the whole payload and inline it into the
// terminal stream; most terminal emulators (and the pseudo-terminal buffers
// they sit behind) start to choke well before tens of megabytes. 5 MiB is a
// conservative middle ground that comfortably covers screenshots, photos and
// typical asset blobs while keeping the worst-case escape-sequence size
// (~7 MiB after base64) within range of common terminal buffers.
const MaxImageBytes = 5 << 20

// renderableMIMETypes lists the raster image MIME types this package is
// willing to hand to a terminal as-is. SVG is intentionally excluded: it is
// text-based XML, neither iTerm2 nor Kitty render it natively, and our cat
// pipeline already routes it through the text rendering path.
var renderableMIMETypes = []string{
	"image/png",
	"image/jpeg",
	"image/gif",
	"image/webp",
	"image/bmp",
	"image/tiff",
}

// kittyNativeMIMETypes is the subset of renderableMIMETypes that the Kitty
// graphics protocol can decode without server-side transcoding. The protocol
// only natively supports raw RGB/RGBA and PNG (f=100); other formats produce
// a black frame or no output at all, so we refuse them on Kitty and let the
// caller fall back to the hex dump.
var kittyNativeMIMETypes = []string{
	"image/png",
}

// ErrUnsupported is returned when the requested protocol cannot render an
// image (either the terminal advertises no image protocol or the payload
// format is not supported by the chosen protocol).
var ErrUnsupported = errors.New("imgview: terminal does not support inline images")

// ErrTooLarge is returned when the payload exceeds MaxImageBytes.
var ErrTooLarge = errors.New("imgview: image payload exceeds size limit")

// IsRenderable reports whether the detected MIME type corresponds to a raster
// image format we are willing to render inline. It is protocol-agnostic; use
// CanRender for a protocol-aware check.
func IsRenderable(m *mime.MIME) bool {
	if m == nil {
		return false
	}
	return slices.ContainsFunc(renderableMIMETypes, m.Is)
}

// CanRender reports whether the given protocol can render the given MIME
// type without server-side transcoding.
func CanRender(proto term.ImageProtocol, m *mime.MIME) bool {
	if m == nil || !proto.Supported() {
		return false
	}
	switch proto {
	case term.ImageKitty:
		return slices.ContainsFunc(kittyNativeMIMETypes, m.Is)
	case term.ImageITerm2:
		return slices.ContainsFunc(renderableMIMETypes, m.Is)
	default:
		return false
	}
}

// Render writes an inline-image escape sequence representing data to w using
// the given protocol. The whole payload is buffered in memory because both
// supported protocols require a single base64 blob.
//
// name is an optional filename hint (used by the iTerm2 protocol; ignored by
// Kitty). Passing the empty string is fine.
func Render(w io.Writer, proto term.ImageProtocol, name string, data []byte) error {
	if len(data) > MaxImageBytes {
		return ErrTooLarge
	}
	switch proto {
	case term.ImageITerm2:
		return renderITerm2(w, name, data)
	case term.ImageKitty:
		return renderKitty(w, data)
	default:
		return ErrUnsupported
	}
}

// Stream reads up to MaxImageBytes from r, then renders the payload using
// Render. size is a best-effort hint used only for buffer pre-allocation; a
// non-positive value disables pre-allocation. If the payload exceeds the
// limit, ErrTooLarge is returned.
//
// The MIME type, if non-nil, is used to derive a filename hint for protocols
// that accept one (iTerm2).
func Stream(w io.Writer, proto term.ImageProtocol, m *mime.MIME, r io.Reader, size int64) error {
	if !CanRender(proto, m) {
		return ErrUnsupported
	}
	buf := bytes.NewBuffer(nil)
	if size > 0 && size <= MaxImageBytes {
		buf.Grow(int(size))
	}
	if _, err := io.Copy(buf, io.LimitReader(r, MaxImageBytes+1)); err != nil {
		return err
	}
	data := buf.Bytes()
	if int64(len(data)) > MaxImageBytes {
		return ErrTooLarge
	}
	name := ""
	if m != nil {
		name = "blob" + m.Extension()
	}
	return Render(w, proto, name, data)
}

// renderITerm2 emits the OSC 1337 inline image sequence understood by
// iTerm2 and WezTerm.
//
//	ESC ] 1337 ; File = [args] : <base64>  BEL
//
// We set inline=1 so the image is shown in place rather than saved, and
// preserveAspectRatio=1 to avoid surprises when the terminal also receives
// width/height hints in a future revision. BEL is used as the terminator
// because tmux passes it through more reliably than the alternative ST.
func renderITerm2(w io.Writer, name string, data []byte) error {
	encoded := base64.StdEncoding.EncodeToString(data)
	args := fmt.Sprintf("size=%s;inline=1;preserveAspectRatio=1", strconv.Itoa(len(data)))
	if name != "" {
		args = fmt.Sprintf("name=%s;%s", base64.StdEncoding.EncodeToString([]byte(name)), args)
	}
	_, err := fmt.Fprintf(w, "\x1b]1337;File=%s:%s\a\n", args, encoded)
	return err
}

// kittyChunkSize is the maximum number of base64 characters per Kitty
// graphics chunk; the protocol mandates chunks of at most 4096 bytes.
const kittyChunkSize = 4096

// renderKitty emits the Kitty graphics protocol "transmit + display" command
// using base64-encoded PNG data, chunked at 4 KiB as required by the spec.
// CanRender is the gatekeeper that ensures we only reach here with PNG data.
//
//	ESC _ G a=T,f=100,m=<0|1> ; <base64-chunk> ESC \   (first chunk)
//	ESC _ G m=<0|1>           ; <base64-chunk> ESC \   (subsequent chunks)
//
// Per the protocol, only the first control block carries the action/format
// keys; subsequent blocks carry only the m (more) flag.
func renderKitty(w io.Writer, data []byte) error {
	encoded := base64.StdEncoding.EncodeToString(data)
	first := true
	for len(encoded) > 0 {
		chunk := encoded
		if len(chunk) > kittyChunkSize {
			chunk = chunk[:kittyChunkSize]
		}
		encoded = encoded[len(chunk):]
		more := 0
		if len(encoded) > 0 {
			more = 1
		}
		var control string
		if first {
			control = fmt.Sprintf("a=T,f=100,m=%d", more)
			first = false
		} else {
			control = fmt.Sprintf("m=%d", more)
		}
		if _, err := fmt.Fprintf(w, "\x1b_G%s;%s\x1b\\", control, chunk); err != nil {
			return err
		}
	}
	// Trailing newline so the next prompt does not glue onto the image row.
	_, err := io.WriteString(w, "\n")
	return err
}
