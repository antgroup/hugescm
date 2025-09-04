package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/antgroup/hugescm/modules/command"
)

type Version struct {
	versionString       string
	major, minor, patch uint32
	rc                  bool
}

// NewVersion constructs a new Git version from the given components.
func NewVersion(major, minor, patch uint32) Version {
	return Version{
		versionString: fmt.Sprintf("%d.%d.%d", major, minor, patch),
		major:         major,
		minor:         minor,
		patch:         patch,
	}
}

// ParseVersionOutput parses output returned by git-version(1). It is expected to be in the format
// "git version 2.39.1".
func ParseVersionOutput(versionOutput []byte) (Version, error) {
	trimmedVersionOutput := string(bytes.Trim(versionOutput, " \n"))
	versionString := strings.SplitN(trimmedVersionOutput, " ", 3)
	if len(versionString) != 3 {
		return Version{}, fmt.Errorf("invalid version format: %q", string(versionOutput))
	}

	version, err := ParseVersion(versionString[2])
	if err != nil {
		return Version{}, fmt.Errorf("cannot parse git version: %w", err)
	}

	return version, nil
}

// String returns the string representation of the version.
func (v Version) String() string {
	return v.versionString
}

// LessThan determines whether the version is older than another version.
func (v Version) LessThan(other Version) bool {
	switch {
	case v.major < other.major:
		return true
	case v.major > other.major:
		return false

	case v.minor < other.minor:
		return true
	case v.minor > other.minor:
		return false

	case v.patch < other.patch:
		return true
	case v.patch > other.patch:
		return false

	case v.rc && !other.rc:
		return true
	case !v.rc && other.rc:
		return false

	default:
		// this should only be reachable when versions are equal
		return false
	}
}

// Equal determines whether the version is the same as another version.
func (v Version) Equal(other Version) bool {
	return v == other
}

// GreaterOrEqual determines whether the version is newer than or equal to another version.
func (v Version) GreaterOrEqual(other Version) bool {
	return !v.LessThan(other)
}

// ParseVersion parses a git version string.
func ParseVersion(versionStr string) (Version, error) {
	versionSplit := strings.SplitN(versionStr, ".", 4)
	if len(versionSplit) < 3 {
		return Version{}, fmt.Errorf("expected major.minor.patch in %q", versionStr)
	}

	ver := Version{
		versionString: versionStr,
	}

	for i, v := range []*uint32{&ver.major, &ver.minor, &ver.patch} {
		var n64 uint64

		if versionSplit[i] == "GIT" {
			// Git falls back to vx.x.GIT if it's unable to describe the current version
			// or if there's a version file. We should just treat this as "0", even
			// though it may have additional commits on top.
			n64 = 0
		} else {
			rcSplit := strings.SplitN(versionSplit[i], "-", 2)

			var err error
			n64, err = strconv.ParseUint(rcSplit[0], 10, 32)
			if err != nil {
				return Version{}, err
			}

			if len(rcSplit) == 2 && strings.HasPrefix(rcSplit[1], "rc") {
				ver.rc = true
			}
		}

		*v = uint32(n64)
	}
	if len(versionSplit) == 4 {
		if strings.HasPrefix(versionSplit[3], "rc") {
			ver.rc = true
		}
	}
	return ver, nil
}

func gitVersionDetect() (Version, error) {
	cmd := command.New(context.Background(), command.NoDir, "git", "version")
	versionOutput, err := cmd.Output()
	if err != nil {
		return Version{}, err
	}
	return ParseVersionOutput(versionOutput)
}

var (
	VersionDetect = sync.OnceValues(gitVersionDetect)
)

// IsVersionAtLeast returns whether the git version is the one specified or higher
// argument is plain version string separated by '.' e.g. "2.3.1" but can omit minor/patch
func IsGitVersionAtLeast(other Version) bool {
	v, err := VersionDetect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting git version: %v\n", err)
		return false
	}
	return v.GreaterOrEqual(other)
}
