package version

import (
	"encoding/json"
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
