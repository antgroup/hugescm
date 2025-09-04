package co

import (
	"fmt"

	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/pkg/version"
)

func NewUserAgent() (string, bool) {
	if !version.TelemetryEnabled() {
		return "", false
	}
	u, err := version.Uname()
	if err != nil {
		return "", false
	}
	v, err := git.VersionDetect()
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("git/%s (%s; %s; %s; %s)", v, u.Node, u.Name, u.Machine, u.Release), true
}
