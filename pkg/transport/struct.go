// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
)

type Operation string

const (
	DOWNLOAD Operation = "download"
	UPLOAD   Operation = "upload"
	SUDO     Operation = "sudo"
)

type SASHandshake struct {
	Operation Operation `json:"operation"`
	Version   string    `json:"version"`
}

type SASPayload struct {
	Header    map[string]string `json:"header,omitempty"`
	Notice    string            `json:"notice,omitempty"`
	ExpiresAt time.Time         `json:"expires_at,omitempty"`
}

func (p *SASPayload) IsExpired() bool {
	return time.Now().After(p.ExpiresAt)
}

type Reference struct {
	Remote          string                 `json:"remote"`
	Name            plumbing.ReferenceName `json:"name"`
	Hash            string                 `json:"hash"`
	Peeled          string                 `json:"peeled,omitempty"`
	HEAD            string                 `json:"head"`
	Version         int                    `json:"version"`
	Agent           string                 `json:"agent"`
	HashAlgo        string                 `json:"hash-algo"`
	CompressionALGO string                 `json:"compression-algo"`
	Capabilities    []string               `json:"capabilities"`
}

func (r *Reference) Target() plumbing.Hash {
	if len(r.Peeled) != 0 {
		return plumbing.NewHash(r.Peeled)
	}
	return plumbing.NewHash(r.Hash)
}

type Command struct {
	Refname     plumbing.ReferenceName `json:"refname"`
	OldRev      string                 `json:"old_rev"`
	NewRev      string                 `json:"new_rev"`
	Metadata    int                    `json:"metadata"`
	Objects     int                    `json:"objects"`
	PushOptions []string               `json:"push_options,omitempty"`
}

type WantObject struct {
	OID string `json:"oid"`
}

type BatchShareObjectsRequest struct {
	Objects []*WantObject `json:"objects"`
}

type Representation struct {
	OID            string            `json:"oid"`
	CompressedSize int64             `json:"compressed_size"`
	Href           string            `json:"href"`
	Header         map[string]string `json:"header,omitempty"`
	ExpiresAt      time.Time         `json:"expires_at,omitempty"`
}

func (r *Representation) IsExpired() bool {
	return time.Now().After(r.ExpiresAt)
}

func (r *Representation) Copy() *Representation {
	header := make(map[string]string)
	if r.Header != nil {
		for k, v := range r.Header {
			header[k] = v
		}
	}
	return &Representation{OID: r.OID, CompressedSize: r.CompressedSize, Href: r.Href, Header: header, ExpiresAt: r.ExpiresAt}
}

type BatchShareObjectsResponse struct {
	Objects []*Representation `json:"objects"`
}

type HaveObject struct {
	OID            string    `json:"oid"`
	CompressedSize int64     `json:"compressed_size"`
	Action         Operation `json:"action,omitempty"`
}

type BatchRequest struct {
	Objects []*HaveObject `json:"objects"`
}

type BatchResponse struct {
	Objects []*HaveObject `json:"objects"`
}
