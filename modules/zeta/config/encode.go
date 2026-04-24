// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/antgroup/hugescm/modules/strengthen"
)

func atomicEncode(zf string, doc Document) error {
	data, err := MarshalDocument(doc)
	if err != nil {
		return err
	}
	return atomicWrite(zf, data)
}

// atomicWrite writes data to a file atomically using write-and-rename pattern.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	_ = os.MkdirAll(dir, 0755)

	cachePath := fmt.Sprintf("%s/.zeta-%d.toml", dir, time.Now().UnixNano())

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return err
	}

	if err := os.Rename(cachePath, path); err != nil {
		_ = os.Remove(cachePath)
		return err
	}
	return nil
}

func Encode(zetaDir string, config *Config) error {
	if config == nil || len(zetaDir) == 0 {
		return ErrInvalidArgument
	}
	zf := filepath.Join(zetaDir, "zeta.toml")
	return atomicWriteConfig(zf, config)
}

func EncodeGlobal(config *Config) error {
	if config == nil {
		return ErrInvalidArgument
	}
	zfg := strengthen.ExpandPath("~/.zeta.toml")
	return atomicWriteConfig(zfg, config)
}

// atomicWriteConfig writes a Config struct to a file atomically using go-toml/v2.
func atomicWriteConfig(path string, config *Config) error {
	var buf bytes.Buffer
	encoder := newTOMLEncoder(&buf)
	if err := encoder.Encode(config); err != nil {
		return err
	}
	return atomicWrite(path, buf.Bytes())
}

type UpdateOptions struct {
	Values map[string]any
	Append bool
}

func updateInternal(zf string, opts *UpdateOptions) error {
	if opts == nil || opts.Values == nil {
		return errors.New("invalid argument for update config")
	}
	// Load existing document or create new one
	doc, err := loadDocumentOrNew(zf)
	if err != nil {
		return err
	}
	// Apply updates
	for k, v := range opts.Values {
		if opts.Append {
			if err := doc.Add(k, v); err != nil {
				return err
			}
		} else {
			if _, err := doc.Set(k, v); err != nil {
				return err
			}
		}
	}
	// Validate before write
	if err := ValidateDocument(doc); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return atomicEncode(zf, doc)
}

// loadDocumentOrNew loads a document from file or returns a new empty document.
func loadDocumentOrNew(path string) (Document, error) {
	doc, err := LoadDocumentFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewDocument(), nil
		}
		return nil, err
	}
	return doc, nil
}

func UpdateSystem(opts *UpdateOptions) error {
	zfg := configSystemPath()
	return updateInternal(zfg, opts)
}

func UpdateGlobal(opts *UpdateOptions) error {
	zfg := strengthen.ExpandPath("~/.zeta.toml")
	return updateInternal(zfg, opts)
}

func UpdateLocal(zetaDir string, opts *UpdateOptions) error {
	zf := filepath.Join(zetaDir, "zeta.toml")
	return updateInternal(zf, opts)
}

func unsetInternal(zf string, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	// Load existing document
	doc, err := LoadDocumentFile(zf)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	// Delete keys
	for _, k := range keys {
		if err := doc.Delete(k); err != nil {
			if err == ErrKeyNotFound {
				continue
			}
			return err
		}
	}
	// Validate before write
	if err := ValidateDocument(doc); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return atomicEncode(zf, doc)
}

func UnsetSystem(keys ...string) error {
	zfg := configSystemPath()
	return unsetInternal(zfg, keys...)
}

func UnsetGlobal(keys ...string) error {
	zfg := strengthen.ExpandPath("~/.zeta.toml")
	return unsetInternal(zfg, keys...)
}

func UnsetLocal(zetaDir string, keys ...string) error {
	zf := filepath.Join(zetaDir, "zeta.toml")
	return unsetInternal(zf, keys...)
}
