// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package odb

import (
	"bytes"
	"context"
	"strings"

	"github.com/antgroup/hugescm/modules/chardet"
	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/plumbing"
)

type MergeDriver func(ctx context.Context, o, a, b string, labelO, labelA, labelB string) (string, bool, error)
type TextGetter func(ctx context.Context, oid plumbing.Hash, textConv bool) (string, string, error)

type mergeOptions struct {
	O, A, B                plumbing.Hash
	LabelO, LableA, LabelB string
	Textconv               bool
	M                      MergeDriver
	G                      TextGetter
}

type mergeTextResult struct {
	oid      plumbing.Hash
	size     int64
	conflict bool
}

func (d *ODB) mergeText(ctx context.Context, opts *mergeOptions) (*mergeTextResult, error) {
	textO, _, err := opts.G(ctx, opts.O, opts.Textconv)
	if err != nil {
		return nil, err
	}
	textA, charset, err := opts.G(ctx, opts.A, opts.Textconv)
	if err != nil {
		return nil, err
	}
	textB, _, err := opts.G(ctx, opts.B, opts.Textconv)
	if err != nil {
		return nil, err
	}
	mergedText, conflict, err := opts.M(ctx, textO, textA, textB, opts.LabelO, opts.LableA, opts.LabelB)
	if err != nil {
		return nil, err
	}
	if !opts.Textconv || strings.EqualFold(charset, diferenco.UTF8) {
		size := int64(len(mergedText))
		oid, err := d.HashTo(ctx, strings.NewReader(mergedText), size)
		if err != nil {
			return nil, err
		}
		return &mergeTextResult{oid: oid, conflict: conflict, size: size}, nil
	}
	restoredText, err := chardet.EncodeToCharset([]byte(mergedText), charset)
	if err != nil {
		return nil, err
	}
	size := int64(len(restoredText))
	oid, err := d.HashTo(ctx, bytes.NewReader(restoredText), size)
	if err != nil {
		return nil, err
	}
	return &mergeTextResult{oid: oid, conflict: conflict, size: size}, nil
}
