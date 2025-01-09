package version

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestGetSystemInfo(t *testing.T) {
	info, err := GetSystemInfo()
	if err != nil {
		return
	}
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", " ")
	_ = enc.Encode(info)
}

func TestUname(t *testing.T) {
	u, err := Uname()
	if err != nil {
		return
	}
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", " ")
	_ = enc.Encode(u)
}

func TestGetUserAgent(t *testing.T) {
	telemetry = "true"
	version = "1.0.0"
	fmt.Fprintf(os.Stderr, "%s\n%s\n", GetUserAgent(), GetBannerVersion())
}
