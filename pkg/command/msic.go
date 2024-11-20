// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/antgroup/hugescm/pkg/kong"
	"github.com/antgroup/hugescm/pkg/tr"
)

var (
	W = tr.W // translate func wrap
)

var (
	ErrSyntaxSize        = errors.New("size synatx error")
	ErrFlagsIncompatible = errors.New("flags incompatible")
)

const (
	Byte int64 = 1 << (iota * 10)
	KiByte
	MiByte
	GiByte
	TiByte
	PiByte
	EiByte
)

var (
	sizeRatio = map[string]int64{
		"b": 1,
		"k": KiByte,
		"m": MiByte,
		"g": GiByte,
		"t": TiByte,
		"p": PiByte,
		"e": EiByte,
	}
)

func decodeSize(text string) (int64, error) {
	text = strings.TrimSuffix(strings.ToLower(text), "b")
	for s, ratio := range sizeRatio {
		if strings.HasSuffix(text, s) {
			i, err := strconv.ParseInt(strings.TrimSpace(text[0:len(text)-len(s)]), 10, 64)
			if err != nil {
				return 0, err
			}
			return i * ratio, nil
		}
	}
	return strconv.ParseInt(text, 10, 64)
}

func SizeDecoder() kong.MapperFunc {
	return func(ctx *kong.DecodeContext, target reflect.Value) error {
		t, err := ctx.Scan.PopValue("string")
		if err != nil {
			return err
		}
		var sv string
		switch v := t.Value.(type) {
		case string:
			sv = v
		default:
			return fmt.Errorf("expected a string value but got %q (%T)", t, t.Value)
		}
		i, err := decodeSize(sv)
		if err != nil {
			return err
		}
		if target.Kind() != reflect.Int64 {
			return fmt.Errorf("internal error: type 'size' only works with fields of type int64; got %s", target.Type())
		}
		target.SetInt(i)
		return nil
	}
}

var (
	typelen = map[string]int64{
		"seconds": 1,
		"minutes": 60,
		"hours":   60 * 60,
		"days":    24 * 60 * 60,
		"weeks":   7 * 24 * 60 * 60,
	}
)

func parseTime(str string) (int64, error) {
	if tt, err := time.Parse(time.RFC3339, str); err == nil {
		d := time.Until(tt)
		return int64(d.Seconds()), nil
	}
	if d, err := time.ParseDuration(str); err == nil {
		return int64(d.Seconds()), nil
	}
	vv := strings.FieldsFunc(str, func(r rune) bool {
		return r == '.' || r == ' '
	})
	if len(vv) != 3 {
		return 0, fmt.Errorf("bad expiry-date %s", str)
	}
	x, err := strconv.ParseInt(vv[0], 10, 64)
	if err != nil {
		return 0, err
	}
	l := typelen[vv[1]]
	if l == 0 {
		return 0, fmt.Errorf("bad expiry-date %s", vv[1])
	}
	return x * l, nil
}

// expiry-date
func ExpiryDateDecoder() kong.MapperFunc {
	return func(ctx *kong.DecodeContext, target reflect.Value) error {
		t, err := ctx.Scan.PopValue("string")
		if err != nil {
			return err
		}
		var sv string
		switch v := t.Value.(type) {
		case string:
			sv = v
		default:
			return fmt.Errorf("expected a string value but got %q (%T)", t, t.Value)
		}
		switch sv {
		case "never", "false":
			target.SetInt(math.MaxInt64)
		case "all", "now":
			target.SetInt(0)
		default:
			t, err := parseTime(sv)
			if err != nil {
				return err
			}
			target.SetInt(t)
		}
		return nil
	}
}

func diev(format string, a ...any) {
	var b bytes.Buffer
	_, _ = b.WriteString(W("fatal: "))
	fmt.Fprintf(&b, W(format), a...)
	_ = b.WriteByte('\n')
	_, _ = os.Stderr.Write(b.Bytes())
}

// die: translate message and fatal
func die(m string) {
	var b bytes.Buffer
	_, _ = b.WriteString(W("fatal: "))
	_, _ = b.WriteString(W(m))
	_ = b.WriteByte('\n')
	_, _ = os.Stderr.Write(b.Bytes())
}

func slashPaths(paths []string) []string {
	newPaths := make([]string, 0, len(paths))
	for _, p := range paths {
		newPaths = append(newPaths, filepath.ToSlash(p))
	}
	return newPaths
}
