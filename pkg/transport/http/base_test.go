package http

import (
	"fmt"
	"mime"
	"os"
	"testing"

	"github.com/antgroup/hugescm/pkg/transport"
)

func TestParseMIME(t *testing.T) {
	ss := []string{
		"application/vnd.zeta+json",
		"application/json",
		"application/json; charset=UTF-8",
	}
	for _, s := range ss {
		m, p, err := mime.ParseMediaType(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse %s error: %v\n", s, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "parse: %s mime: %s param: %v\n", s, m, p)
	}

}

func TestRangeHeader(t *testing.T) {
	sa := &transport.SASPayload{}
	for k, v := range sa.Header {
		fmt.Fprintf(os.Stderr, "%v %v\n", k, v)
	}
}
