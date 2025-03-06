// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"math"
	"time"
)

const (
	PROTOCOL_Z1             = "Z1"
	PROTOCOL_VERSION uint32 = 1
	// references prefix
	REF_PREFIX    = "refs/"
	BRANCH_PREFIX = "refs/heads/" // branch prefix
	TAG_PREFIX    = "refs/tags/"  // tag prefix
	HEAD          = "HEAD"
	// MIME
	ZETA_MIME_MD          = "application/x-zeta-metadata"
	ZETA_MIME_COMPRESS_MD = "application/x-zeta-compress-metadata"
	// other
	MAX_BATCH_BLOB_SIZE = math.MaxUint32 - 64
)

var (
	metaTransportMagic    = [4]byte{'Z', 'M', '\x00', '\x01'}
	objectsTransportMagic = [4]byte{'Z', 'B', '\x00', '\x02'}
	reserved              [16]byte // reserved zero fill
)

type Operation string

const (
	PSEUDO   Operation = ""
	DOWNLOAD Operation = "download"
	UPLOAD   Operation = "upload"
	SUDO     Operation = "sudo"
)

type SASHandshake struct {
	Operation Operation `json:"operation"`
	Version   string    `json:"version,omitempty"`
}

type PayloadHeader struct {
	Authorization string `json:"authorization"`
}

type SASPayload struct {
	Header    PayloadHeader `json:"header"`
	Notice    string        `json:"notice,omitempty"`
	ExpiresAt time.Time     `json:"expires_at,omitzero"`
}

type ErrorCode struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (e *ErrorCode) Error() string {
	return e.Message
}

type Reference struct {
	Remote          string   `json:"remote"`
	Name            string   `json:"name"`
	Hash            string   `json:"hash"`
	Peeled          string   `json:"peeled,omitempty"`
	HEAD            string   `json:"head"`
	Version         int      `json:"version"`
	Agent           string   `json:"agent"`
	HashAlgo        string   `json:"hash-algo"`
	CompressionAlgo string   `json:"compression-algo"`
	Capabilities    []string `json:"capabilities"`
}

type Branch struct {
	Remote          string   `json:"remote"`
	Branch          string   `json:"branch"`
	Hash            string   `json:"hash"`
	Version         int      `json:"version"`
	Agent           string   `json:"agent"`
	HashAlgo        string   `json:"hash-algo"`
	DefaultBranch   string   `json:"default-branch"`
	CompressionAlgo string   `json:"compression-algo"`
	Capabilities    []string `json:"capabilities"`
}

type Tag struct {
	Remote          string   `json:"remote"`
	Tag             string   `json:"tag"`
	Hash            string   `json:"hash"`
	Version         int      `json:"version"`
	Agent           string   `json:"agent"`
	HashAlgo        string   `json:"hash-algo"`
	CompressionAlgo string   `json:"compression-algo"`
	Capabilities    []string `json:"capabilities"`
}

type WantObject struct {
	OID  string `json:"oid"`
	Path string `json:"path,omitempty"`
}

type BatchShareObjectsRequest struct {
	Objects []*WantObject `json:"objects"`
}

type Representation struct {
	OID            string            `json:"oid"`
	CompressedSize int64             `json:"compressed_size"`
	Href           string            `json:"href"`
	Header         map[string]string `json:"header,omitempty"`
	ExpiresAt      time.Time         `json:"expires_at,omitzero"`
}

type BatchShareObjectsResponse struct {
	Objects []*Representation `json:"objects"`
}

type HaveObject struct {
	OID            string `json:"oid"`
	CompressedSize int64  `json:"compressed_size"`
	Action         string `json:"action,omitempty"`
}

type BatchCheckRequest struct {
	Objects []*HaveObject `json:"objects"`
}

type BatchCheckResponse struct {
	Objects []*HaveObject `json:"objects"`
}
