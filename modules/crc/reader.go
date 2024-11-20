package crc

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"hash/crc64"
	"io"
	"strings"
)

type Crc64Writer struct {
	io.Writer
	Base io.Writer
	h    hash.Hash
}

type Finisher interface {
	Finish() (string, error)
}

func NewCrc64Writer(w io.Writer) *Crc64Writer {
	h := crc64.New(crc64.MakeTable(crc64.ISO))
	return &Crc64Writer{
		Writer: io.MultiWriter(w, h),
		Base:   w,
		h:      h,
	}
}

func (cw *Crc64Writer) Finish() (string, error) {
	if cw.h == nil {
		return "", nil
	}
	checksum := hex.EncodeToString(cw.h.Sum(nil))
	if _, err := cw.Write([]byte(checksum)); err != nil {
		return "", errors.New("write checksum error")
	}
	return checksum, nil
}

type Crc64Reader struct {
	br *bufio.Reader
	h  hash.Hash
}

func (cr *Crc64Reader) Read(p []byte) (n int, err error) {
	n, err = cr.br.Read(p)
	if err == nil {
		cr.h.Write(p[:n])
	}
	return
}

func NewCrc64Reader(r io.Reader) *Crc64Reader {
	return &Crc64Reader{br: bufio.NewReader(r), h: crc64.New(crc64.MakeTable(crc64.ISO))}
}

func (cr *Crc64Reader) Verify() error {
	var sum [16]byte
	if _, err := io.ReadFull(cr.br, sum[:]); err != nil {
		return err
	}
	want := string(sum[:])
	got := hex.EncodeToString(cr.h.Sum(nil))
	if strings.EqualFold(got, want) {
		return nil
	}
	return fmt.Errorf("unexpected crc64 checksum got '%s' want '%s'", got, want)
}
