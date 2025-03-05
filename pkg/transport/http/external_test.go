package http

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/antgroup/hugescm/pkg/transport"
)

func TestProxy(t *testing.T) {
	dl := NewDownloader(true, false, "socks5://127.0.0.1:13659")
	sr, err := dl.Download(t.Context(), &transport.Representation{
		Href: "https://github.com/zed-industries/zed/releases/download/v0.166.1/zed-remote-server-macos-x86_64.gz",
	}, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "download error: %v\n", err)
		return
	}
	defer sr.Close()
	if _, err := io.Copy(io.Discard, sr); err != nil {
		fmt.Fprintf(os.Stderr, "download error: %v\n", err)
	}
}
