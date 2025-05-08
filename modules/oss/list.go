// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oss

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/url"
	"time"
)

// ListObjectsResult defines the result from ListObjects request
type ListObjectsResult struct {
	XMLName        xml.Name           `xml:"ListBucketResult"`
	Prefix         string             `xml:"Prefix"`                // The object prefix
	Marker         string             `xml:"Marker"`                // The marker filter.
	MaxKeys        int                `xml:"MaxKeys"`               // Max keys to return
	Delimiter      string             `xml:"Delimiter"`             // The delimiter for grouping objects' name
	IsTruncated    bool               `xml:"IsTruncated"`           // Flag indicates if all results are returned (when it's false)
	NextMarker     string             `xml:"NextMarker"`            // The start point of the next query
	Objects        []ObjectProperties `xml:"Contents"`              // Object list
	CommonPrefixes []string           `xml:"CommonPrefixes>Prefix"` // You can think of commonprefixes as "folders" whose names end with the delimiter
}

// ObjectProperties defines Object properties
type ObjectProperties struct {
	XMLName      xml.Name  `xml:"Contents"`
	Key          string    `xml:"Key"`                   // Object key
	Type         string    `xml:"Type"`                  // Object type
	Size         int64     `xml:"Size"`                  // Object size
	ETag         string    `xml:"ETag"`                  // Object ETag
	Owner        Owner     `xml:"Owner"`                 // Object owner information
	LastModified time.Time `xml:"LastModified"`          // Object last modified time
	StorageClass string    `xml:"StorageClass"`          // Object storage class (Standard, IA, Archive)
	RestoreInfo  string    `xml:"RestoreInfo,omitempty"` // Object restoreInfo
}

// ListObjectsResultV2 defines the result from ListObjectsV2 request
type ListObjectsResultV2 struct {
	XMLName               xml.Name           `xml:"ListBucketResult"`
	Prefix                string             `xml:"Prefix"`                // The object prefix
	StartAfter            string             `xml:"StartAfter"`            // the input StartAfter
	ContinuationToken     string             `xml:"ContinuationToken"`     // the input ContinuationToken
	MaxKeys               int                `xml:"MaxKeys"`               // Max keys to return
	Delimiter             string             `xml:"Delimiter"`             // The delimiter for grouping objects' name
	IsTruncated           bool               `xml:"IsTruncated"`           // Flag indicates if all results are returned (when it's false)
	NextContinuationToken string             `xml:"NextContinuationToken"` // The start point of the next NextContinuationToken
	Objects               []ObjectProperties `xml:"Contents"`              // Object list
	CommonPrefixes        []string           `xml:"CommonPrefixes>Prefix"` // You can think of commonprefixes as "folders" whose names end with the delimiter
}

type Object struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
	ETag string `json:"etag"`
}

const (
	MaxKeys = 1000
)

// https://www.alibabacloud.com/help/zh/oss/developer-reference/listobjectsv2
func (b *bucket) ListObjects(ctx context.Context, prefix, continuationToken string) ([]*Object, string, error) {
	q := make(url.Values)
	q.Set("list-type", "2")
	q.Set("max-keys", "1000")
	q.Set("prefix", prefix)
	if len(continuationToken) != 0 {
		q.Set("continuation-token", continuationToken)
	}
	qs := q.Encode()
	u := &url.URL{
		Scheme:   b.scheme,
		Host:     b.bucketEndpoint,
		RawQuery: qs,
	}
	req, err := b.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, "", err
	}
	resource := b.getResourceV2("", qs)
	b.signature(req, resource)
	resp, err := b.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close() // nolint
	if resp.StatusCode == http.StatusNotFound {
		return nil, "", readOssError(resp)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", readOssError(resp)
	}
	var result ListObjectsResultV2
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", err
	}
	objects := make([]*Object, 0, len(result.Objects))
	for _, o := range result.Objects {
		objects = append(objects, &Object{Key: o.Key, Size: o.Size, ETag: o.ETag})
	}
	return objects, result.NextContinuationToken, nil
}
