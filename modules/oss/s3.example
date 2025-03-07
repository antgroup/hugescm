// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oss

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type s3Bucket struct {
	Bucket
	client     *s3.Client
	pc         *s3.PresignClient
	bucketName string
	partSize   int64
}

func NewS3Bucket(ctx context.Context, s3Region, s3AccessKeyID, s3AccessKeySecret, s3BucketName string, partSize int64) (Bucket, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(s3Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(s3AccessKeyID, s3AccessKeySecret, "")))
	if err != nil {
		return nil, err
	}
	if partSize <= minPartSize {
		partSize = defaultPartSize
	}

	// Create an Amazon S3 service client
	client := s3.NewFromConfig(cfg)

	return &s3Bucket{client: client, pc: s3.NewPresignClient(client), bucketName: s3BucketName, partSize: partSize}, nil
}

func (b *s3Bucket) Stat(ctx context.Context, resourcePath string) (*Stat, error) {
	o, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(resourcePath),
	})
	if err != nil {
		return nil, err
	}
	return &Stat{
		Size: aws.ToInt64(o.ContentLength),
		Mime: aws.ToString(o.ContentType),
	}, nil
}

func (b *s3Bucket) checkSize(ctx context.Context, resourcePath string, o *s3.GetObjectOutput) (int64, error) {
	if rangeHdr := aws.ToString(o.ContentRange); len(rangeHdr) != 0 {
		if size, err := parseSizeFromRange(rangeHdr); err == nil {
			return size, nil
		}
		si, err := b.Stat(ctx, resourcePath)
		if err != nil {
			return 0, err
		}
		return si.Size, nil
	}
	return aws.ToInt64(o.ContentLength), nil
}

func (b *s3Bucket) Open(ctx context.Context, resourcePath string, start, length int64) (RangeReader, error) {
	var awsRange *string
	switch {
	case start < 0:
		awsRange = aws.String(fmt.Sprintf("bytes=%d", start))
	case start >= 0 && length > 0:
		awsRange = aws.String(fmt.Sprintf("bytes=%d-%d", start, start+length-1))
	case start > 0:
		awsRange = aws.String(fmt.Sprintf("bytes=%d-", start))
	default: // NO RANGE
	}
	o, err := b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(resourcePath),
		Range:  awsRange,
	})
	if err != nil {
		return nil, err
	}
	size, err := b.checkSize(ctx, resourcePath, o)
	if err != nil {
		_ = o.Body.Close()
		return nil, err
	}
	return NewRangeReader(o.Body, size, aws.ToString(o.ContentRange)), nil
}

func (b *s3Bucket) Delete(ctx context.Context, resourcePath string) error {
	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(resourcePath),
	})
	return err
}

func (b *s3Bucket) Put(ctx context.Context, resourcePath string, r io.Reader, mime string) error {
	_, err := b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(b.bucketName),
		Key:         aws.String(resourcePath),
		ContentType: aws.String(mime),
		Body:        r,
	})
	if err != nil {
		return err
	}
	return nil
}

func (b *s3Bucket) upload(ctx context.Context, resourcePath, filePath string, mime string) error {
	fd, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer fd.Close()
	return b.Put(ctx, resourcePath, fd, mime)
}

func (b *s3Bucket) uploadFilePart(ctx context.Context, resourcePath string, filePath string, mur *s3.CreateMultipartUploadOutput, k chunk) (types.CompletedPart, error) {
	result := types.CompletedPart{PartNumber: aws.Int32(int32(k.number))}
	fd, err := os.Open(filePath)
	if err != nil {
		return result, err
	}
	defer fd.Close()
	if _, err := fd.Seek(k.offset, io.SeekStart); err != nil {
		return result, err
	}
	return b.uploadPart(ctx, resourcePath, io.LimitReader(fd, k.size), mur, k)
}

func (b *s3Bucket) uploadPart(ctx context.Context, resourcePath string, reader io.Reader, mur *s3.CreateMultipartUploadOutput, k chunk) (types.CompletedPart, error) {
	result := types.CompletedPart{PartNumber: aws.Int32(int32(k.number))}
	o, err := b.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:   aws.String(b.bucketName),
		Key:      aws.String(resourcePath),
		UploadId: mur.UploadId,
		Body:     reader,
	})
	if err != nil {
		return result, err
	}
	result.ETag = o.ETag
	return result, nil
}

func (b *s3Bucket) StartUpload(ctx context.Context, resourcePath, filePath string, mime string) error {
	si, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("stat file error: %w", err)
	}
	size := si.Size()
	if size <= b.partSize {
		return b.upload(ctx, resourcePath, filePath, mime)
	}
	chunks := calculateChunk(size, b.partSize)
	if len(chunks) < 2 {
		return fmt.Errorf("BUGS BAD CHUNK. size: %d, len(chunks): %d", size, len(chunks))
	}

	mur, err := b.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(b.bucketName),
		Key:         aws.String(resourcePath),
		ContentType: aws.String(mime),
	})
	if err != nil {
		return err
	}
	newCtx, cancelCtx := context.WithCancel(ctx)
	// defer cancelCtx()
	results := make(chan types.CompletedPart, len(chunks))
	failed := make(chan error)
	for i := 0; i < len(chunks); i++ {
		go func(k chunk) {
			u, err := b.uploadFilePart(newCtx, resourcePath, filePath, mur, k)
			if err != nil {
				failed <- fmt.Errorf("upload part-%d error: %w", k.number, err)
				return
			}
			results <- u
		}(chunks[i])
	}
	parts := make([]types.CompletedPart, len(chunks))
	completed := 0
	for completed < len(chunks) {
		select {
		case part := <-results:
			completed++
			parts[aws.ToInt32(part.PartNumber)-1] = part
		case err := <-failed:
			cancelCtx()
			_, _ = b.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(b.bucketName),
				Key:      aws.String(resourcePath),
				UploadId: mur.UploadId,
			})
			return err
		}
		if completed >= len(chunks) {
			break
		}
	}
	cancelCtx()
	if _, err := b.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(resourcePath),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: parts,
		},
		UploadId: mur.UploadId,
	}); err != nil {
		_, _ = b.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(b.bucketName),
			Key:      aws.String(resourcePath),
			UploadId: mur.UploadId,
		})
		return fmt.Errorf("complete upload error: %w", err)
	}
	return nil
}

func (b *s3Bucket) LinearUpload(ctx context.Context, resourcePath string, r io.Reader, size int64, mime string) error {
	if size < maxPartSize {
		return b.Put(ctx, resourcePath, r, mime)
	}
	chunks := calculateChunk(size, b.partSize)
	if len(chunks) < 2 {
		return fmt.Errorf("BUGS BAD CHUNK. size: %d, len(chunks): %d", size, len(chunks))
	}
	mur, err := b.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(b.bucketName),
		Key:         aws.String(resourcePath),
		ContentType: aws.String(mime),
	})
	if err != nil {
		return err
	}
	parts := make([]types.CompletedPart, len(chunks))
	for i, k := range chunks {
		u, err := b.uploadPart(ctx, resourcePath, io.LimitReader(r, k.size), mur, k)
		if err != nil {
			_, _ = b.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(b.bucketName),
				Key:      aws.String(resourcePath),
				UploadId: mur.UploadId,
			})
			return err
		}
		parts[i] = u
	}
	if _, err := b.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(resourcePath),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: parts,
		},
		UploadId: mur.UploadId,
	}); err != nil {
		_, _ = b.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(b.bucketName),
			Key:      aws.String(resourcePath),
			UploadId: mur.UploadId,
		})
		return fmt.Errorf("complete upload error: %w", err)
	}
	return nil
}

func (b *s3Bucket) DeleteMultipleObjects(ctx context.Context, objectKeys []string) error {
	d := &types.Delete{}
	for _, o := range objectKeys {
		d.Objects = append(d.Objects, types.ObjectIdentifier{
			Key: aws.String(o),
		})
	}
	_, err := b.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(b.bucketName),
		Delete: d,
	})
	return err
}

func (b *s3Bucket) ListObjects(ctx context.Context, prefix, continuationToken string) ([]*Object, string, error) {
	in := &s3.ListObjectsV2Input{
		Bucket:  aws.String(b.bucketName),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1000),
	}
	if len(continuationToken) != 0 {
		in.ContinuationToken = aws.String(continuationToken)
	}
	out, err := b.client.ListObjectsV2(ctx, in)
	if err != nil {
		return nil, "", err
	}
	objects := make([]*Object, 0, len(out.Contents))
	for _, o := range out.Contents {
		objects = append(objects, &Object{Key: aws.ToString(o.Key), Size: aws.ToInt64(o.Size), ETag: aws.ToString(o.ETag)})
	}
	return objects, aws.ToString(out.ContinuationToken), nil
}

func (b *s3Bucket) Share(ctx context.Context, resourcePath string, expiresAt int64) string {
	o, err := b.pc.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucketName),
		Key:    aws.String(resourcePath),
	}, func(po *s3.PresignOptions) {
		po.Expires = time.Second * time.Duration(expiresAt)
	})
	if err != nil {
		return ""
	}
	return o.URL
}
