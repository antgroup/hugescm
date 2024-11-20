// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oss

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// https://help.aliyun.com/document_detail/386432.htm?spm=a2c4g.475520.0.0.2c8bc7c3AkNfW5

// https://help.aliyun.com/document_detail/31951.html?spm=a2c4g.31955.4.5.27b86cf05lSqjf&scm=20140722.H_31951._.ID_31951-OR_rec-V_1
// Authorization = "OSS " + AccessKeyId + ":" + Signature
// Signature = base64(hmac-sha1(AccessKeySecret,
//             VERB + "\n"
//             + Content-MD5 + "\n"
//             + Content-Type + "\n"
//             + Date + "\n"
//             + CanonicalizedOSSHeaders
//             + CanonicalizedResource))

// CanonicalizedResource
// https://help.aliyun.com/document_detail/31951.html?spm=a2c4g.31955.4.5.27b86cf05lSqjf&scm=20140722.H_31951._.ID_31951-OR_rec-V_1#section-rvv-dx2-xdb

// CanonicalizedOSSHeaders
// https://help.aliyun.com/document_detail/31951.html?spm=a2c4g.31955.4.5.27b86cf05lSqjf&scm=20140722.H_31951._.ID_31951-OR_rec-V_1#section-w2k-sw2-xdb

// headerSorter defines the key-value structure for storing the sorted data in signHeader.
type headerSorter struct {
	Keys []string
	Vals []string
}

// newHeaderSorter is an additional function for function SignHeader.
func newHeaderSorter(m map[string]string) *headerSorter {
	hs := &headerSorter{
		Keys: make([]string, 0, len(m)),
		Vals: make([]string, 0, len(m)),
	}

	for k, v := range m {
		hs.Keys = append(hs.Keys, k)
		hs.Vals = append(hs.Vals, v)
	}
	return hs
}

// Sort is an additional function for function SignHeader.
func (hs *headerSorter) Sort() {
	sort.Sort(hs)
}

// Len is an additional function for function SignHeader.
func (hs *headerSorter) Len() int {
	return len(hs.Vals)
}

// Less is an additional function for function SignHeader.
func (hs *headerSorter) Less(i, j int) bool {
	return bytes.Compare([]byte(hs.Keys[i]), []byte(hs.Keys[j])) < 0
}

// Swap is an additional function for function SignHeader.
func (hs *headerSorter) Swap(i, j int) {
	hs.Vals[i], hs.Vals[j] = hs.Vals[j], hs.Vals[i]
	hs.Keys[i], hs.Keys[j] = hs.Keys[j], hs.Keys[i]
}

// NewSignature creates signature for string following Aliyun rules
func NewSignature(content, accessKeySecret string) string {
	// Crypto by HMAC-SHA256
	h := hmac.New(sha256.New, []byte(accessKeySecret))
	h.Write([]byte(content))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// additionalList, _ := conn.getAdditionalHeaderKeys(req)
// if len(additionalList) > 0 {
// 	authorizationFmt := "OSS2 AccessKeyId:%v,AdditionalHeaders:%v,Signature:%v"
// 	additionnalHeadersStr := strings.Join(additionalList, ";")
// 	authorizationStr = fmt.Sprintf(authorizationFmt, akIf.GetAccessKeyID(), additionnalHeadersStr, conn.getSignedStr(req, canonicalizedResource, akIf.GetAccessKeySecret()))
// } else {
// 	authorizationFmt := "OSS2 AccessKeyId:%v,Signature:%v"
// 	authorizationStr = fmt.Sprintf(authorizationFmt, akIf.GetAccessKeyID(), conn.getSignedStr(req, canonicalizedResource, akIf.GetAccessKeySecret()))
// }

func (b *bucket) signature(req *http.Request, canonicalizedResource string) {
	req.Header.Set("x-oss-signature-version", "OSS2")
	now := time.Now().UTC()
	req.Header.Set("Date", now.Format(http.TimeFormat))
	// Find out the "x-oss-"'s address in header of the request
	headers := make(map[string]string)
	for k, v := range req.Header {
		k = strings.ToLower(k)
		if strings.HasPrefix(k, "x-oss-") {
			headers[k] = v[0]
		}
	}
	hs := newHeaderSorter(headers)
	hs.Sort()
	var cw strings.Builder
	for i := range hs.Keys {
		_, _ = cw.WriteString(hs.Keys[i])
		_ = cw.WriteByte(':')
		_, _ = cw.WriteString(hs.Vals[i])
		_ = cw.WriteByte('\n')
	}
	date := req.Header.Get("Date")
	contentType := req.Header.Get("Content-Type")
	contentMd5 := req.Header.Get("Content-MD5")

	h := hmac.New(sha256.New, []byte(b.accessKeySecret))
	signedText := req.Method + "\n" + contentMd5 + "\n" + contentType + "\n" + date + "\n" + cw.String() + "\n" + canonicalizedResource
	_, _ = h.Write([]byte(signedText))
	signed := base64.StdEncoding.EncodeToString(h.Sum(nil))
	authorizationStr := fmt.Sprintf("OSS2 AccessKeyId:%v,Signature:%v", b.accessKeyID, signed)
	req.Header.Set("Authorization", authorizationStr)
}

func (b *bucket) getResourceV2(objectName, subResource string) string {
	if subResource != "" {
		subResource = "?" + subResource
	}
	return url.QueryEscape("/"+b.name+"/") + strings.Replace(url.QueryEscape(objectName), "+", "%20", -1) + subResource
}
