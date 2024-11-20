package ssh

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/antgroup/hugescm/modules/shlex"
)

func TestEscapeArgs(t *testing.T) {
	vv, err := shlex.Split("zeta-serve ls-remote '--reference=refs/heads/jack' 'repo/jack~1'", true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "EEE %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "args: %v\n", strings.Join(vv, ","))
}
