// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/antgroup/hugescm/pkg/serve/protocol"
	"github.com/sirupsen/logrus"
)

const (
	ErrorMessageKey = "X-Zeta-Error-Message"
	JSON_MIME       = "application/json"
)

// ResponseWriter shadow ResponseWriter
type ResponseWriter struct {
	http.ResponseWriter
	written    int64
	statusCode int
	remoteAddr string
}

// NewResponseWriter bind ResponseWriter
func NewResponseWriter(w http.ResponseWriter, r *http.Request) *ResponseWriter {
	return &ResponseWriter{ResponseWriter: w, statusCode: http.StatusOK, remoteAddr: parseRemoteAddress(r)}
}

// Write data
func (w *ResponseWriter) Write(data []byte) (int, error) {
	written, err := w.ResponseWriter.Write(data)
	w.written += int64(written)
	return written, err
}

// WriteHeader write header statusCode
func (w *ResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// StatusCode return statusCode
func (w *ResponseWriter) StatusCode() int {
	return w.statusCode
}

// Written return body size
func (w *ResponseWriter) Written() int64 {
	return w.written
}

func (w *ResponseWriter) F1RemoteAddr() string {
	return w.remoteAddr
}

type trackedReader struct {
	rc       io.ReadCloser
	received int64
}

func newTrackedReader(rc io.ReadCloser) *trackedReader {
	return &trackedReader{rc: rc}
}

// Read reads up to len(data) bytes from the channel.
func (r *trackedReader) Read(data []byte) (int, error) {
	n, err := r.rc.Read(data)
	r.received += int64(n)
	return n, err
}

func (r *trackedReader) Close() error {
	return r.rc.Close()
}

func parseRemoteAddress(r *http.Request) string {
	addr := strings.TrimSpace(r.Header.Get("X-Zeta-Effective-IP"))
	if len(addr) != 0 {
		return addr
	}
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if addr = strings.TrimSpace(strings.Split(xForwardedFor, ",")[0]); len(addr) != 0 {
		return addr
	}

	if addr = strings.TrimSpace(r.Header.Get("X-Real-Ip")); len(addr) != 0 {
		return addr
	}
	addr, _, _ = net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	return addr
}

func renderFailureFormat(w http.ResponseWriter, r *http.Request, code int, format string, a ...any) {
	resp := &protocol.ErrorCode{
		Code:    code,
		Message: fmt.Sprintf(format, a...),
	}
	w.Header().Set("Content-Type", JSON_MIME)
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(resp)
	if code != 200 {
		r.Header.Set(ErrorMessageKey, resp.Message)
	}
}
func renderFailure(w http.ResponseWriter, r *http.Request, code int, message string) {
	resp := &protocol.ErrorCode{
		Code:    code,
		Message: message,
	}
	w.Header().Set("Content-Type", JSON_MIME)
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(resp)
	if code != 200 {
		r.Header.Set(ErrorMessageKey, message)
	}
}

func (s *Server) renderErrorRaw(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case plumbing.IsNoSuchObject(err), plumbing.IsErrRevNotFound(err):
		renderFailure(w, r, http.StatusNotFound, err.Error())
	case os.IsNotExist(err), database.IsNotFound(err), object.IsErrDirectoryNotFound(err), object.IsErrEntryNotFound(err):
		renderFailureFormat(w, r, http.StatusNotFound, "resource not found: %v", err)
	case backend.IsErrMismatchedObjectType(err), database.IsErrExist(err), errors.Is(err, fs.ErrExist):
		renderFailure(w, r, http.StatusConflict, err.Error())
	default:
		renderFailure(w, r, http.StatusInternalServerError, "internal server error")
		r.Header.Set(ErrorMessageKey, err.Error())
	}
}

func (s *Server) renderError(w http.ResponseWriter, r *Request, err error) {
	switch {
	case plumbing.IsNoSuchObject(err), plumbing.IsErrRevNotFound(err):
		renderFailure(w, r.Request, http.StatusNotFound, err.Error())
	case os.IsNotExist(err), database.IsNotFound(err), object.IsErrDirectoryNotFound(err), object.IsErrEntryNotFound(err):
		renderFailureFormat(w, r.Request, http.StatusNotFound, "resource not found: %v", err)
	case backend.IsErrMismatchedObjectType(err), database.IsErrExist(err), os.IsExist(err):
		renderFailure(w, r.Request, http.StatusConflict, err.Error())
	default:
		renderFailure(w, r.Request, http.StatusInternalServerError, r.W("internal server error"))
		r.Header.Set(ErrorMessageKey, err.Error())
	}
}

func JsonEncode(w http.ResponseWriter, a any) {
	// RFC https://www.rfc-editor.org/rfc/rfc8259.html#section-8.1
	// JSON text exchanged between systems that are not part of a closed
	// ecosystem MUST be encoded using UTF-8 [RFC3629].

	// Previous specifications of JSON have not required the use of UTF-8
	// when transmitting JSON text.  However, the vast majority of JSON-
	// based software implementations have chosen to use the UTF-8 encoding,
	// to the extent that it is the only encoding that achieves
	// interoperability.

	// The media type for JSON text is application/json.

	w.Header().Set("Content-Type", JSON_MIME)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(a); err != nil {
		logrus.Errorf("encode response error: %v", err)
	}
}

func ZetaEncodeVND(w http.ResponseWriter, a any) {
	// RFC https://www.rfc-editor.org/rfc/rfc8259.html#section-8.1
	// JSON text exchanged between systems that are not part of a closed
	// ecosystem MUST be encoded using UTF-8 [RFC3629].

	// Previous specifications of JSON have not required the use of UTF-8
	// when transmitting JSON text.  However, the vast majority of JSON-
	// based software implementations have chosen to use the UTF-8 encoding,
	// to the extent that it is the only encoding that achieves
	// interoperability.

	// The media type for JSON text is application/json.

	w.Header().Set("Content-Type", ZETA_MIME_VND_JSON)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(a); err != nil {
		logrus.Errorf("encode response error: %v", err)
	}
}
