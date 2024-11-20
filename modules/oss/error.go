// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oss

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
)

// Error represents an error in an operation with OSS.
type Error struct {
	StatusCode int    // HTTP status code (200, 403, ...)
	Code       string // OSS error code ("UnsupportedOperation", ...)
	Message    string // The human-oriented error message
	BucketName string
	RequestId  string
	HostId     string
}

func (e *Error) Error() string {
	return fmt.Sprintf("Aliyun API Error: RequestId: %s Status Code: %d Code: %s Message: %s", e.RequestId, e.StatusCode, e.Code, e.Message)
}

// ServiceError contains fields of the error response from Oss Service REST API.
type ServiceError struct {
	XMLName    xml.Name `xml:"Error"`
	Code       string   `xml:"Code"`      // The error code returned from OSS to the caller
	Message    string   `xml:"Message"`   // The detail error message from OSS
	RequestID  string   `xml:"RequestId"` // The UUID used to uniquely identify the request
	HostID     string   `xml:"HostId"`    // The OSS server cluster's Id
	Endpoint   string   `xml:"Endpoint"`
	Ec         string   `xml:"EC"`
	RawMessage string   // The raw messages from OSS
	StatusCode int      // HTTP status code

}

// Error implements interface error
func (e *ServiceError) Error() string {
	errorMessage := fmt.Sprintf("oss: service returned error: StatusCode=%d, ErrorCode=%s, ErrorMessage=\"%s\", RequestId=%s", e.StatusCode, e.Code, e.Message, e.RequestID)
	if len(e.Endpoint) > 0 {
		errorMessage = fmt.Sprintf("%s, Endpoint=%s", errorMessage, e.Endpoint)
	}
	if len(e.Ec) > 0 {
		errorMessage = fmt.Sprintf("%s, Ec=%s", errorMessage, e.Ec)
	}
	return errorMessage
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	out, err := io.ReadAll(resp.Body)
	if err == io.EOF {
		err = nil
	}
	return out, err
}

func serviceErrFromXML(body []byte, statusCode int, requestID string) (*ServiceError, error) {
	var se ServiceError

	if err := xml.Unmarshal(body, &se); err != nil {
		return nil, err
	}

	se.StatusCode = statusCode
	se.RequestID = requestID
	se.RawMessage = string(body)
	return &se, nil
}

func readOssError(resp *http.Response) error {
	if resp.StatusCode >= 400 && resp.StatusCode <= 505 {
		b, err := readResponseBody(resp)
		if err != nil {
			return err
		}
		if len(b) == 0 && len(resp.Header.Get("X-Oss-Err")) != 0 {
			if e, err := base64.StdEncoding.DecodeString(resp.Header.Get("X-Oss-Err")); err == nil {
				b = e
			}
		}
		if len(b) > 0 {
			if se, err := serviceErrFromXML(b, resp.StatusCode, resp.Header.Get("X-Oss-Request-Id")); err == nil {
				return se
			}
		}
	}
	return &ServiceError{StatusCode: resp.StatusCode, RequestID: resp.Header.Get("X-Oss-Request-Id"), Ec: resp.Header.Get("X-Oss-Ec")}
}
