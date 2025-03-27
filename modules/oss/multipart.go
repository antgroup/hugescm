// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oss

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"time"
)

// size constant defined
const (
	Byte int64 = 1 << (iota * 10)
	KiByte
	MiByte
	GiByte
	TiByte
	PiByte
	EiByte
)

const (
	MaxRecvBytes = 16 << 20 // 16M
	MaxSendBytes = math.MaxInt32
)

const (
	// https://help.aliyun.com/document_detail/31850.html?spm=a2c4g.31847.0.0.71f013681jxCO0
	minPartSize     = 100 * 1024
	maxPartSize     = 5 * GiByte
	defaultPartSize = GiByte
	// MaxPartSize                 = 5 * 1024 * 1024 * 1024 // Max part size, 5GB
	// MinPartSize                 = 100 * 1024             // Min part size, 100KB
)

// InitiateMultipartUploadResult defines result of InitiateMultipartUpload request
type InitiateMultipartUploadResult struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	Bucket   string   `xml:"Bucket"`   // Bucket name
	Key      string   `xml:"Key"`      // Object name to upload
	UploadID string   `xml:"UploadId"` // Generated UploadId
}

// UploadPart defines the upload/copy part
type UploadPart struct {
	XMLName    xml.Name `xml:"Part"`
	PartNumber int      `xml:"PartNumber"` // Part number
	ETag       string   `xml:"ETag"`       // ETag value of the part's data
}

type completeMultipartUploadXML struct {
	XMLName xml.Name     `xml:"CompleteMultipartUpload"`
	Part    []UploadPart `xml:"Part"`
}

// CompleteMultipartUploadResult defines result object of CompleteMultipartUploadRequest
type CompleteMultipartUploadResult struct {
	XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
	Location string   `xml:"Location"` // Object URL
	Bucket   string   `xml:"Bucket"`   // Bucket name
	ETag     string   `xml:"ETag"`     // Object ETag
	Key      string   `xml:"Key"`      // Object name
}

type UploadParts []UploadPart

func (slice UploadParts) Len() int {
	return len(slice)
}

func (slice UploadParts) Less(i, j int) bool {
	return slice[i].PartNumber < slice[j].PartNumber
}

func (slice UploadParts) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

type chunk struct {
	number int   // chunk number
	offset int64 // chunk offset
	size   int64 // chunk size
}

func calculateChunk(size, partSize int64) []chunk {
	if size%partSize < minPartSize {
		partSize -= minPartSize
	}
	N := int(size / partSize)
	chunks := make([]chunk, 0, N+1)
	var offset int64
	for i := range N {
		chunks = append(chunks, chunk{number: i + 1, offset: offset, size: partSize})
		offset += partSize
	}
	if offset < size {
		chunks = append(chunks, chunk{number: N + 1, offset: offset, size: size - offset})
	}
	return chunks
}

/*

<?xml version="1.0" encoding="UTF-8"?>
<InitiateMultipartUploadResult xmlns=”http://doc.oss-cn-hangzhou.aliyuncs.com”>
    <Bucket> oss-example</Bucket>
    <Key>multipart.data</Key>
    <UploadId>0004B9894A22E5B1888A1E29F823****</UploadId>
</InitiateMultipartUploadResult>

*/

// InitiateMultipartUpload
// https://www.alibabacloud.com/help/en/object-storage-service/latest/initiatemultipartupload
func (b *bucket) initiateMultipartUpload(ctx context.Context, resourcePath string, mime string) (*InitiateMultipartUploadResult, error) {
	q := "uploads"
	u := &url.URL{
		Scheme:   b.scheme,
		Host:     b.bucketEndpoint,
		Path:     resourcePath,
		RawQuery: q,
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), nil)
	if err != nil {
		return nil, err
	}
	if len(mime) != 0 {
		req.Header.Set("Content-Type", mime)
	}
	resource := b.getResourceV2(resourcePath, q)
	b.signature(req, resource)
	resp, err := b.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // nolint
	if resp.StatusCode == http.StatusNotFound {
		return nil, os.ErrNotExist
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, readOssError(resp)
	}
	var result InitiateMultipartUploadResult
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// https://www.alibabacloud.com/help/en/object-storage-service/latest/abortmultipartupload
func (b *bucket) abortMultipartUpload(resourcePath string, mur *InitiateMultipartUploadResult) error {
	// NOTE: If the upload fails due to context cancellation, we cannot use the original context because that would cause our cleanup to fail.
	ctx, cancelCtx := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelCtx()
	q := fmt.Sprintf("uploadId=%s", mur.UploadID)
	u := &url.URL{
		Scheme:   b.scheme,
		Host:     b.bucketEndpoint,
		Path:     resourcePath,
		RawQuery: q,
	}
	req, err := http.NewRequestWithContext(ctx, "DELETE", u.String(), nil)
	if err != nil {
		return err
	}
	resource := b.getResourceV2(resourcePath, q)
	b.signature(req, resource)
	resp, err := b.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() // nolint
	if resp.StatusCode == http.StatusNotFound {
		return readOssError(resp)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return errors.New(resp.Status)
	}
	return nil
}

// https://www.alibabacloud.com/help/en/object-storage-service/latest/completemultipartupload
func (b *bucket) completeMultipartUpload(ctx context.Context, resourcePath string, mur *InitiateMultipartUploadResult, uploadParts []UploadPart) error {
	sort.Sort(UploadParts(uploadParts))
	q := fmt.Sprintf("uploadId=%s", mur.UploadID)
	u := &url.URL{
		Scheme:   b.scheme,
		Host:     b.bucketEndpoint,
		Path:     resourcePath,
		RawQuery: q,
	}
	input := &completeMultipartUploadXML{Part: uploadParts}
	body, err := xml.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	resource := b.getResourceV2(resourcePath, q)
	b.signature(req, resource)
	resp, err := b.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() // nolint
	if resp.StatusCode == http.StatusNotFound {
		return readOssError(resp)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return errors.New(resp.Status)
	}
	var result CompleteMultipartUploadResult
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	return nil
}

// https://www.alibabacloud.com/help/en/object-storage-service/latest/uploadpart
func (b *bucket) uploadPart(ctx context.Context, resourcePath string, reader io.Reader, mur *InitiateMultipartUploadResult, k chunk) (UploadPart, error) {
	result := UploadPart{PartNumber: k.number}
	q := fmt.Sprintf("partNumber=%d&uploadId=%s", k.number, mur.UploadID)
	u := &url.URL{
		Scheme:   b.scheme,
		Host:     b.bucketEndpoint,
		Path:     resourcePath,
		RawQuery: q,
	}
	req, err := http.NewRequestWithContext(ctx, "PUT", u.String(), reader)
	if err != nil {
		return result, err
	}
	resource := b.getResourceV2(resourcePath, q)
	b.signature(req, resource)
	resp, err := b.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close() // nolint
	if resp.StatusCode == http.StatusNotFound {
		return result, os.ErrNotExist
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return result, readOssError(resp)
	}
	result.ETag = resp.Header.Get("ETag")
	return result, nil
}

func (b *bucket) LinearUpload(ctx context.Context, resourcePath string, r io.Reader, size int64, mime string) error {
	if size < maxPartSize {
		return b.Put(ctx, resourcePath, r, mime)
	}
	chunks := calculateChunk(size, b.partSize)
	if len(chunks) < 2 {
		return fmt.Errorf("BUGS BAD CHUNK. size: %d, len(chunks): %d", size, len(chunks))
	}
	mur, err := b.initiateMultipartUpload(ctx, resourcePath, mime)
	if err != nil {
		return err
	}
	parts := make([]UploadPart, len(chunks))
	for i, k := range chunks {
		u, err := b.uploadPart(ctx, resourcePath, io.LimitReader(r, k.size), mur, k)
		if err != nil {
			_ = b.abortMultipartUpload(resourcePath, mur)
			return err
		}
		parts[i] = u
	}
	if err := b.completeMultipartUpload(ctx, resourcePath, mur, parts); err != nil {
		_ = b.abortMultipartUpload(resourcePath, mur)
		return fmt.Errorf("complete upload error: %w", err)
	}
	return nil
}
