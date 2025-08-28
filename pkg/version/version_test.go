package version

import (
	"fmt"
	"os"
	"testing"
)

func TestGetUserAgent(t *testing.T) {
	telemetry = "true"
	version = "1.27"
	fmt.Fprintf(os.Stderr, "%s\n", GetUserAgent())
}

func TestGetBannerVersion(t *testing.T) {
	telemetry = "true"
	version = "1.27"
	fmt.Fprintf(os.Stderr, "%s\n", GetBannerVersion())
}
