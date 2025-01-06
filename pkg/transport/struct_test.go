package transport

import (
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRepresentationExpired(t *testing.T) {
	r := &Representation{
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	fmt.Fprintf(os.Stderr, "IsExpired: %v\n", r.IsExpired())
}

func TestRepresentationExpired2(t *testing.T) {
	r := &Representation{
		ExpiresAt: time.Now().Add(time.Hour),
	}
	fmt.Fprintf(os.Stderr, "IsExpired: %v\n", r.IsExpired())
}

func TestTokenExpired(t *testing.T) {
	r := &SASPayload{}
	fmt.Fprintf(os.Stderr, "IsExpired: %v\n", r.IsExpired())
}

func TestPathJoin(t *testing.T) {
	u, err := url.Parse("https://zeta.example.io/sigma/konfig-dev")
	require.NoError(t, err)
	u2 := u.JoinPath("reference", "refs/heads/master--dev")
	fmt.Fprintf(os.Stderr, "%s\n", u2.String())
	u3, err := url.Parse("https://zeta.example.io/sigma/konfig-dev/reference/refs/heads/master+dev")
	require.NoError(t, err)
	fmt.Fprintf(os.Stderr, "%s\n", u3.Path)
}

func TestEndpointIsURL(t *testing.T) {
	sss := []string{
		"http://xxxx",
		"git@xxxx",
		"zeta@zzzz",
	}
	for _, s := range sss {
		fmt.Fprintf(os.Stderr, "%s %v\n", s, hasScheme(s))
	}
}

func TestParseEndpoint(t *testing.T) {
	sss := []string{
		"http://zeta.io/jack/zeta-demo",
		"https://zeta.io/jack/zeta-demo",
		"zeta@zeta.io:jack/zeta-demo",
		"ssh://zeta@zeta.io/jack/zeta-demo",
	}
	for _, s := range sss {
		e, err := NewEndpoint(s, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Parse: %v\n", err)
			continue
		}
		fmt.Fprintf(os.Stderr, "endpoint: %v protocol: %s\n", e, e.Protocol)
	}
}
