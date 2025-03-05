package systemproxy

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestDialGithub(t *testing.T) {
	var d net.Dialer
	proxyURL, err := url.Parse("http://127.0.0.1:8080")
	if err != nil {
		return
	}
	conn, err := DialServerViaCONNECT(t.Context(), "github.com:22", proxyURL, &d)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("SSH-2.0-Jack-7.9\n")); err != nil {
		fmt.Fprintf(os.Stderr, "write error: %v\n", err)
		return
	}

	br := bufio.NewReader(conn)
	line, _, err := br.ReadLine()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ReadLine error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "line: %s\n", strings.TrimSpace(string(line)))
}
