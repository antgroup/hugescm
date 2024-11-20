// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oss

import (
	"context"
	"fmt"
	"io"
	"os"
)

// upload without multipart
func (b *bucket) upload(ctx context.Context, resourcePath, filePath string, mime string) error {
	fd, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer fd.Close()
	return b.Put(ctx, resourcePath, fd, mime)
}

func (b *bucket) uploadFilePart(ctx context.Context, resourcePath string, filePath string, mur *InitiateMultipartUploadResult, k chunk) (UploadPart, error) {
	result := UploadPart{PartNumber: k.number}
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

func (b *bucket) StartUpload(ctx context.Context, resourcePath, filePath string, mime string) error {
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
	mur, err := b.initiateMultipartUpload(ctx, resourcePath, mime)
	if err != nil {
		return err
	}
	newCtx, cancelCtx := context.WithCancel(ctx)
	// defer cancelCtx()
	results := make(chan UploadPart, len(chunks))
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
	parts := make([]UploadPart, len(chunks))
	completed := 0
	for completed < len(chunks) {
		select {
		case part := <-results:
			completed++
			parts[part.PartNumber-1] = part
		case err := <-failed:
			cancelCtx()
			_ = b.abortMultipartUpload(resourcePath, mur)
			return err
		}
		if completed >= len(chunks) {
			break
		}
	}
	cancelCtx()
	if err := b.completeMultipartUpload(ctx, resourcePath, mur, parts); err != nil {
		_ = b.abortMultipartUpload(resourcePath, mur)
		return fmt.Errorf("complete upload error: %w", err)
	}
	return nil
}
