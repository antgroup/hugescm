// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oss

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	escQuot = []byte("&#34;") // shorter than "&quot;"
	escApos = []byte("&#39;") // shorter than "&apos;"
	escAmp  = []byte("&amp;")
	escLT   = []byte("&lt;")
	escGT   = []byte("&gt;")
	escTab  = []byte("&#x9;")
	escNL   = []byte("&#xA;")
	escCR   = []byte("&#xD;")
	escFFFD = []byte("\uFFFD") // Unicode replacement character
)

func EscapeLFString(str string) string {
	var log bytes.Buffer
	for i := 0; i < len(str); i++ {
		if str[i] != '\n' {
			log.WriteByte(str[i])
		} else {
			log.WriteString("\\n")
		}
	}
	return log.String()
}

// EscapeString writes to p the properly escaped XML equivalent
// of the plain text data s.
func EscapeXml(s string) string {
	var p strings.Builder
	var esc []byte
	hextable := "0123456789ABCDEF"
	escPattern := []byte("&#x00;")
	last := 0
	for i := 0; i < len(s); {
		r, width := utf8.DecodeRuneInString(s[i:])
		i += width
		switch r {
		case '"':
			esc = escQuot
		case '\'':
			esc = escApos
		case '&':
			esc = escAmp
		case '<':
			esc = escLT
		case '>':
			esc = escGT
		case '\t':
			esc = escTab
		case '\n':
			esc = escNL
		case '\r':
			esc = escCR
		default:
			if !isInCharacterRange(r) || (r == 0xFFFD && width == 1) {
				if r >= 0x00 && r < 0x20 {
					escPattern[3] = hextable[r>>4]
					escPattern[4] = hextable[r&0x0f]
					esc = escPattern
				} else {
					esc = escFFFD
				}
				break
			}
			continue
		}
		p.WriteString(s[last : i-width])
		p.Write(esc)
		last = i
	}
	p.WriteString(s[last:])
	return p.String()
}

// Decide whether the given rune is in the XML Character Range, per
// the Char production of https://www.xml.com/axml/testaxml.htm,
// Section 2.2 Characters.
func isInCharacterRange(r rune) (inrange bool) {
	return r == 0x09 ||
		r == 0x0A ||
		r == 0x0D ||
		r >= 0x20 && r <= 0xD7FF ||
		r >= 0xE000 && r <= 0xFFFD ||
		r >= 0x10000 && r <= 0x10FFFF
}

type deleteXML struct {
	XMLName xml.Name        `xml:"Delete"`
	Objects []*DeleteObject `xml:"Object"` // Objects to delete
	Quiet   bool            `xml:"Quiet"`  // Flag of quiet mode.
}

// DeleteObject defines the struct for deleting object
type DeleteObject struct {
	XMLName   xml.Name `xml:"Object"`
	Key       string   `xml:"Key"`                 // Object name
	VersionId string   `xml:"VersionId,omitempty"` // Object VersionId
}

// DeleteObjectsResult defines result of DeleteObjects request
type DeleteObjectsResult struct {
	XMLName        xml.Name
	DeletedObjects []string // Deleted object key list
}

// DeletedKeyInfo defines object delete info
type DeletedKeyInfo struct {
	XMLName               xml.Name `xml:"Deleted"`
	Key                   string   `xml:"Key"`                   // Object key
	VersionId             string   `xml:"VersionId"`             // VersionId
	DeleteMarker          bool     `xml:"DeleteMarker"`          // Object DeleteMarker
	DeleteMarkerVersionId string   `xml:"DeleteMarkerVersionId"` // Object DeleteMarkerVersionId
}

type DeleteObjectVersionsResult struct {
	XMLName              xml.Name         `xml:"DeleteResult"`
	DeletedObjectsDetail []DeletedKeyInfo `xml:"Deleted"` // Deleted object detail info
}

// Owner defines Bucket/Object's owner
type Owner struct {
	XMLName     xml.Name `xml:"Owner"`
	ID          string   `xml:"ID"`          // Owner ID
	DisplayName string   `xml:"DisplayName"` // Owner's display name
}

// marshalDeleteObjectToXml deleteXML struct to xml
func marshalDeleteObjectToXml(dxml deleteXML) string {
	var builder strings.Builder
	builder.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	builder.WriteString("<Delete>")
	builder.WriteString("<Quiet>")
	builder.WriteString(strconv.FormatBool(dxml.Quiet))
	builder.WriteString("</Quiet>")
	if len(dxml.Objects) > 0 {
		for _, object := range dxml.Objects {
			builder.WriteString("<Object>")
			if object.Key != "" {
				builder.WriteString("<Key>")
				builder.WriteString(EscapeXml(object.Key))
				builder.WriteString("</Key>")
			}
			if object.VersionId != "" {
				builder.WriteString("<VersionId>")
				builder.WriteString(object.VersionId)
				builder.WriteString("</VersionId>")
			}
			builder.WriteString("</Object>")
		}
	}
	builder.WriteString("</Delete>")
	return builder.String()
}

// https://www.alibabacloud.com/help/zh/oss/developer-reference/deleteobject
func (b *bucket) Delete(ctx context.Context, resourcePath string) error {
	u := &url.URL{
		Scheme: b.scheme,
		Host:   b.bucketEndpoint,
		Path:   resourcePath,
	}
	req, err := b.NewRequestWithContext(ctx, "DELETE", u.String(), nil)
	if err != nil {
		return err
	}
	resource := b.getResourceV2(resourcePath, "")
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

func (b *bucket) deleteMultipleObjects(ctx context.Context, objectKeys []string) error {
	var dxml deleteXML
	for _, key := range objectKeys {
		dxml.Objects = append(dxml.Objects, &DeleteObject{Key: key})
	}
	xmlData := marshalDeleteObjectToXml(dxml)
	q := "delete"
	u := &url.URL{
		Scheme:   b.scheme,
		Host:     b.bucketEndpoint,
		RawQuery: q,
	}
	md5sum := md5.Sum([]byte(xmlData))
	req, err := b.NewRequestWithContext(ctx, "POST", u.String(), strings.NewReader(xmlData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Content-MD5", base64.StdEncoding.EncodeToString(md5sum[:]))
	resource := b.getResourceV2("", q)
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
		return readOssError(resp)
	}
	var result DeleteObjectVersionsResult
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	return nil
}

// https://www.alibabacloud.com/help/zh/oss/developer-reference/deletemultipleobjects
func (b *bucket) DeleteMultipleObjects(ctx context.Context, objectKeys []string) error {
	for len(objectKeys) > 0 {
		minSize := min(len(objectKeys), 200)
		if err := b.deleteMultipleObjects(ctx, objectKeys[:minSize]); err != nil {
			return err
		}
		objectKeys = objectKeys[minSize:]
	}
	return nil
}
