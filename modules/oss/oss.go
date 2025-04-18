// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oss

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultContentSha256 = "UNSIGNED-PAYLOAD" // for v4 signature
	OssContentSha256Key  = "X-Oss-Content-Sha256"
)

// PutObject https://help.aliyun.com/document_detail/31978.htm?spm=a2c4g.31948.0.0.3ec1f0355LA8x4#reference-l5p-ftw-tdb
// GetObject https://help.aliyun.com/document_detail/31980.htm?spm=a2c4g.31948.0.0.3ec1f0355LA8x4#reference-ccf-rgd-5db
// HeadObject https://help.aliyun.com/document_detail/31984.htm?spm=a2c4g.31948.0.0.3ec1f0355LA8x4#reference-bgh-cbw-wdb
// GetObjectMeta https://help.aliyun.com/document_detail/31985.htm?spm=a2c4g.31948.0.0.3ec1f0355LA8x4#reference-sg4-k2w-wdb
// DeleteObject https://help.aliyun.com/document_detail/31982.htm?spm=a2c4g.31948.0.0.3ec1f0355LA8x4#reference-iqc-mqv-wdb

type Bucket interface {
	Stat(ctx context.Context, resourcePath string) (*Stat, error)
	Open(ctx context.Context, resourcePath string, start, length int64) (RangeReader, error)
	Delete(ctx context.Context, resourcePath string) error
	Put(ctx context.Context, resourcePath string, r io.Reader, mime string) error
	StartUpload(ctx context.Context, resourcePath, filePath string, mime string) error
	// LinearUpload: Aliyun oss currently has a 5GB file upload limit, so when the OSS object exceeds 5GB, we use the MultipartUpload mechanism to upload. However,
	// please note that due to network failures or other problems, large file uploads are prone to failure, and LFS is currently not working well. scheme to solve this problem.
	LinearUpload(ctx context.Context, resourcePath string, r io.Reader, size int64, mime string) error
	DeleteMultipleObjects(ctx context.Context, objectKeys []string) error
	ListObjects(ctx context.Context, prefix, continuationToken string) ([]*Object, string, error)
	Share(ctx context.Context, resourcePath string, expiresAt int64) string
}

var (
	_ Bucket = &bucket{}
)

const (
	defaultConnTimeout           = time.Second * 60
	defaultReadWriteTimeout      = time.Second * 120
	defaultIdleConnTimeout       = time.Second * 100
	defaultResponseHeaderTimeout = time.Second * 120
	defaultMaxIdleConns          = 100
	defaultMaxIdleConnsPerHost   = 100
)

type bucket struct {
	scheme               string
	host                 string
	name                 string
	accessKeyID          string // AccessId
	accessKeySecret      string // AccessKey
	bucketEndpoint       string
	sharedScheme         string
	sharedBucketEndpoint string
	product              string
	region               string
	partSize             int64 // upload file multipart size
	*http.Client
}

type NewBucketOptions struct {
	Endpoint        string
	SharedEndpoint  string
	Bucket          string
	AccessKeyID     string
	AccessKeySecret string
	Product         string
	Region          string
	PartSize        int64
}

func NewBucket(opts *NewBucketOptions) (Bucket, error) {
	endpoint := opts.Endpoint
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	b := &bucket{
		scheme:          u.Scheme,
		host:            u.Host,
		name:            opts.Bucket,
		accessKeyID:     opts.AccessKeyID,
		accessKeySecret: opts.AccessKeySecret,
		bucketEndpoint:  opts.Bucket + "." + u.Host,
		product:         opts.Product,
		region:          opts.Region,
		partSize:        opts.PartSize,
		Client: &http.Client{
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				DialContext:         dialer.DialContext,
				ForceAttemptHTTP2:   true,
				MaxIdleConns:        defaultMaxIdleConns,
				MaxIdleConnsPerHost: defaultMaxIdleConnsPerHost,
				IdleConnTimeout:     defaultIdleConnTimeout,
			},
		}}
	if b.partSize <= 0 {
		b.partSize = defaultPartSize
	}
	if len(opts.SharedEndpoint) == 0 {
		b.sharedScheme = b.scheme
		b.sharedBucketEndpoint = b.bucketEndpoint
		return b, nil
	}
	sharedEndpoint := opts.SharedEndpoint
	if !strings.Contains(sharedEndpoint, "://") {
		sharedEndpoint = "http://" + sharedEndpoint
	}
	sharedURL, err := url.Parse(sharedEndpoint)
	if err != nil {
		return nil, err
	}
	b.sharedScheme = sharedURL.Scheme
	b.sharedBucketEndpoint = opts.Bucket + "." + sharedURL.Host
	return b, nil
}

type Stat struct {
	Size  int64
	Mime  string
	Crc64 string
}
