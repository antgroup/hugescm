// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"time"
)

const (
	// GitTimeLayout is the (default) time layout used by git.
	GitTimeLayout = "Mon Jan _2 15:04:05 2006 -0700"
)

// Signature represents the Author or Committer information.
type Signature struct {
	// Name represents a person name. It is an arbitrary string.
	Name string
	// Email is an email, but it cannot be assumed to be well-formed.
	Email string
	// When is the timestamp of the signature.
	When time.Time
}

const (
	formatTimeZoneOnly = "-0700"
)

// String implements the fmt.Stringer interface and formats a Signature as
// expected in the Git commit internal object format. For instance:
//
//	Taylor Blau <ttaylorr@github.com> 1494258422 -0600
func (s *Signature) String() string {
	at := s.When.Unix()
	zone := s.When.Format(formatTimeZoneOnly)

	return fmt.Sprintf("%s <%s> %d %s", s.Name, s.Email, at, zone)
}

// Decode decodes a byte array representing a signature to signature
func (s *Signature) Decode(b []byte) {
	sig, _ := newSignatureFromCommitLine(b)
	s.Email = sig.Email
	s.Name = sig.Name
	s.When = sig.When
}

// Helper to get a signature from the commit line, which looks like these:
//
//	author Patrick Gundlach <gundlach@speedata.de> 1378823654 +0200
//	author Patrick Gundlach <gundlach@speedata.de> Thu, 07 Apr 2005 22:13:13 +0200
//
// but without the "author " at the beginning (this method should)
// be used for author and committer.
func newSignatureFromCommitLine(line []byte) (sig *Signature, err error) {
	sig = new(Signature)
	emailStart := bytes.LastIndexByte(line, '<')
	emailEnd := bytes.LastIndexByte(line, '>')
	if emailStart == -1 || emailEnd == -1 || emailEnd < emailStart {
		return
	}

	sig.Name = string(line[:emailStart-1])
	sig.Email = string(line[emailStart+1 : emailEnd])

	hasTime := emailEnd+2 < len(line)
	if !hasTime {
		return
	}

	// Check date format.
	firstChar := line[emailEnd+2]
	if firstChar >= 48 && firstChar <= 57 {
		idx := bytes.IndexByte(line[emailEnd+2:], ' ')
		if idx < 0 {
			return
		}

		timestring := string(line[emailEnd+2 : emailEnd+2+idx])
		seconds, _ := strconv.ParseInt(timestring, 10, 64)
		sig.When = time.Unix(seconds, 0)

		idx += emailEnd + 3
		if idx >= len(line) || idx+5 > len(line) {
			return
		}

		timezone := string(line[idx : idx+5])
		tzhours, err1 := strconv.ParseInt(timezone[0:3], 10, 64)
		tzmins, err2 := strconv.ParseInt(timezone[3:], 10, 64)
		if err1 != nil || err2 != nil {
			return
		}
		if tzhours < 0 {
			tzmins *= -1
		}
		tz := time.FixedZone("", int(tzhours*60*60+tzmins*60))
		sig.When = sig.When.In(tz)
	} else {
		sig.When, err = time.Parse(GitTimeLayout, string(line[emailEnd+2:]))
		if err != nil {
			return
		}
	}
	return sig, err
}

func (s *Signature) Encode(w io.Writer) error {
	if _, err := fmt.Fprintf(w, "%s <%s> ", s.Name, s.Email); err != nil {
		return err
	}
	if err := s.encodeTimeAndTimeZone(w); err != nil {
		return err
	}
	return nil
}

func (s *Signature) encodeTimeAndTimeZone(w io.Writer) error {
	u := s.When.Unix()
	if u < 0 {
		u = 0
	}
	_, err := fmt.Fprintf(w, "%d %s", u, s.When.Format("-0700"))
	return err
}

func SignatureFromLine(line string) *Signature {
	if signature, err := newSignatureFromCommitLine([]byte(line)); err == nil {
		return signature
	}
	return &Signature{}
}
