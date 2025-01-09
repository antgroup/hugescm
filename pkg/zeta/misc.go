// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/vfs"
	"github.com/antgroup/hugescm/modules/wildmatch"
	"github.com/antgroup/hugescm/pkg/tr"
	"github.com/antgroup/hugescm/pkg/zeta/odb"
	"github.com/mattn/go-isatty"
)

const (
	extremeSize                        = 50 << 20 // 50M
	ENV_ZETA_CORE_ACCELERATOR          = "ZETA_CORE_ACCELERATOR"
	ENV_ZETA_CORE_OPTIMIZE_STRATEGY    = "ZETA_CORE_OPTIMIZE_STRATEGY"
	ENV_ZETA_CORE_CONCURRENT_TRANSFERS = "ZETA_CORE_CONCURRENT_TRANSFERS"
	ENV_ZETA_CORE_SHARING_ROOT         = "ZETA_CORE_SHARING_ROOT"
	ENV_ZETA_CORE_PROMISOR             = "ZETA_CORE_PROMISOR"
	ENV_ZETA_AUTHOR_NAME               = "ZETA_AUTHOR_NAME"
	ENV_ZETA_AUTHOR_EMAIL              = "ZETA_AUTHOR_EMAIL"
	ENV_ZETA_AUTHOR_DATE               = "ZETA_AUTHOR_DATE"
	ENV_ZETA_COMMITTER_NAME            = "ZETA_COMMITTER_NAME"
	ENV_ZETA_COMMITTER_EMAIL           = "ZETA_COMMITTER_EMAIL"
	ENV_ZETA_COMMITTER_DATE            = "ZETA_COMMITTER_DATE"
	ENV_ZETA_MERGE_TEXT_DRIVER         = "ZETA_MERGE_TEXT_DRIVER"
	ENV_ZETA_EDITOR                    = "ZETA_EDITOR"
	ENV_ZETA_SSL_NO_VERIFY             = "ZETA_SSL_NO_VERIFY"
	ENV_ZETA_TRANSPORT_MAX_ENTRIES     = "ZETA_TRANSPORT_MAX_ENTRIES"
	ENV_ZETA_TRANSPORT_LARGE_SIZE      = "ZETA_TRANSPORT_LARGE_SIZE"
	ENV_ZETA_TRANSPORT_EXTERNAL_PROXY  = "ZETA_TRANSPORT_EXTERNAL_PROXY"
)

var (
	is256ColorSupported  bool
	isTrueColorSupported bool
	W                    = tr.W // translate func wrap
)

func IsTerminal(fd uintptr) bool {
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func init() {
	stdout := os.Stdout.Fd()
	if !isatty.IsTerminal(stdout) && !isatty.IsCygwinTerminal(stdout) {
		return
	}
	if _, ok := os.LookupEnv("WT_SESSION"); ok {
		is256ColorSupported = true
		isTrueColorSupported = true
		return
	}
	colorTermEnv := os.Getenv("COLORTERM")
	termEnv := os.Getenv("TERM")
	isTrueColorSupported = strings.Contains(termEnv, "24bit") ||
		strings.Contains(termEnv, "truecolor") ||
		strings.Contains(colorTermEnv, "24bit") ||
		strings.Contains(colorTermEnv, "truecolor")
	is256ColorSupported = isTrueColorSupported || strings.Contains(termEnv, "256") || strings.Contains(colorTermEnv, "256")
}

// ErrNotExist commit not exist error
type ErrNotZetaDir struct {
	cwd string
}

func (err *ErrNotZetaDir) Error() string {
	return fmt.Sprintf("'%s' %s", err.cwd, W("not zeta repository"))
}

func IsErrNotZetaDir(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ErrNotZetaDir)
	return ok
}

func checkDestination(repoName, destination string, mustEmpty bool) (string, bool, error) {
	if len(destination) == 0 {
		destination = repoName
	}
	if !filepath.IsAbs(destination) {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Get current workdir error: %v\n", err)
			return "", false, err
		}
		destination = filepath.Join(cwd, destination)
	}
	dirs, err := os.ReadDir(destination)
	if err != nil {
		if os.IsNotExist(err) {
			return destination, false, nil
		}
		fmt.Fprintf(os.Stderr, "readdir %s error: %v\n", destination, err)
		return "", false, err
	}
	if len(dirs) != 0 && mustEmpty {
		die_error("destination path '%s' already exists and is not an empty directory.", filepath.Base(destination))
		return "", false, ErrWorktreeNotEmpty
	}
	return destination, true, nil
}

// FindZetaDir return worktreeDir, zetaDir, err
func FindZetaDir(cwd string) (string, string, error) {
	var err error
	if len(cwd) == 0 {
		if cwd, err = os.Getwd(); err != nil {
			return "", "", err
		}
	}
	current, err := filepath.Abs(cwd)
	if err != nil {
		return "", "", err
	}
	for {
		if odb.IsZetaDir(current) {
			return filepath.Dir(current), current, nil
		}
		currentZetaDir := filepath.Join(current, ".zeta")
		if odb.IsZetaDir(currentZetaDir) {
			return current, currentZetaDir, nil
		}
		parent := filepath.Dir(current)
		if current == parent {
			return "", "", &ErrNotZetaDir{cwd: cwd}
		}
		current = parent
	}
}

func (r *Repository) DbgPrint(format string, args ...any) {
	if !r.verbose {
		return
	}
	message := fmt.Sprintf(format, args...)
	var buffer bytes.Buffer
	for _, s := range strings.Split(message, "\n") {
		_, _ = buffer.WriteString("\x1b[33m* ")
		_, _ = buffer.WriteString(s)
		_, _ = buffer.WriteString("\x1b[0m\n")
	}
	_, _ = os.Stderr.Write(buffer.Bytes())
}

func (r *Repository) Debug(format string, args ...any) {
	if r.quiet {
		return
	}
	fmt.Fprintf(os.Stderr, format, args...)
}

type Matcher struct {
	prefix     []string
	wildmatchs []*wildmatch.Wildmatch
}

func NewMatcher(patterns []string) *Matcher {
	m := &Matcher{}
	for _, pattern := range patterns {
		if len(pattern) == 0 {
			continue
		}
		if !strings.ContainsAny(pattern, escapeChars) {
			m.prefix = append(m.prefix, strings.TrimSuffix(pattern, "/"))
			continue
		}
		m.wildmatchs = append(m.wildmatchs, wildmatch.NewWildmatch(pattern, wildmatch.SystemCase, wildmatch.Contents))
	}
	return m
}

func (m *Matcher) FsMatch(fs vfs.VFS) error {
	for _, p := range m.prefix {
		if _, err := fs.Lstat(p); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("pathspec '%s' did not match any files", p)
			}
			return err
		}
	}
	return nil
}

func hasDotDot(name string) bool {
	return name == dotDot || (strings.HasPrefix(name, dotDot) && name[2] == '/')
}

func (m *Matcher) Match(name string) bool {
	if len(m.wildmatchs) == 0 && len(m.prefix) == 0 {
		return true
	}
	for _, p := range m.prefix {
		prefixLen := len(p)
		if len(name) >= prefixLen && systemCaseEqual(name[0:prefixLen], p) && (len(name) == prefixLen || name[prefixLen] == '/') {
			return true
		}
	}
	for _, w := range m.wildmatchs {
		if w.Match(name) {
			return true
		}
	}
	return false
}

var (
	caseInsensitive = func() bool {
		return runtime.GOOS == "windows" || runtime.GOOS == "darwin"
	}()
)

func canonicalName(name string) string {
	if caseInsensitive {
		return strings.ToLower(name)
	}
	return name
}

func systemCaseEqual(a, b string) bool {
	if caseInsensitive {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func shortHash(h plumbing.Hash) string {
	return h.String()[0:8]
}

func die(format string, a ...any) {
	var b bytes.Buffer
	_, _ = b.WriteString(W("fatal: "))
	fmt.Fprintf(&b, W(format), a...)
	_ = b.WriteByte('\n')
	_, _ = os.Stderr.Write(b.Bytes())
}

func dieln(a ...any) {
	var b bytes.Buffer
	_, _ = b.WriteString(W("fatal: "))
	fmt.Fprintln(&b, a...)
	_ = b.WriteByte('\n')
	_, _ = os.Stderr.Write(b.Bytes())
}

func die_error(format string, a ...any) {
	var b bytes.Buffer
	_, _ = b.WriteString(W("error: "))
	fmt.Fprintf(&b, W(format), a...)
	_ = b.WriteByte('\n')
	_, _ = os.Stderr.Write(b.Bytes())
}

func error_red(format string, args ...any) {
	prefix := W("error: ")
	message := strings.TrimSuffix(fmt.Sprintf(W(format), args...), "\n")
	var b bytes.Buffer
	switch {
	case is256ColorSupported:
		for _, s := range strings.Split(message, "\n") {
			_, _ = b.WriteString("\x1b[31m")
			_, _ = b.WriteString(prefix)
			_, _ = b.WriteString(s)
			_, _ = b.WriteString("\x1b[0m\n")
		}
	case isTrueColorSupported:
		for _, s := range strings.Split(message, "\n") {
			_, _ = b.WriteString("\x1b[31m")
			_, _ = b.WriteString(prefix)
			_, _ = b.WriteString(s)
			_, _ = b.WriteString("\x1b[0m\n")
		}
	default:
		for _, s := range strings.Split(message, "\n") {
			_, _ = b.WriteString(prefix)
			_, _ = b.WriteString(s)
			_ = b.WriteByte('\n')
		}
	}
	_, _ = os.Stderr.Write(b.Bytes())
}

func warn(format string, a ...any) {
	var b bytes.Buffer
	_, _ = b.WriteString(W("warning: "))
	fmt.Fprintf(&b, W(format), a...)
	_ = b.WriteByte('\n')
	_, _ = os.Stderr.Write(b.Bytes())
}

type ErrExitCode struct {
	ExitCode int
	Message  string
}

func IsExitCode(err error, i int) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*ErrExitCode); ok {
		return e.ExitCode == i
	}
	return false
}

func (e *ErrExitCode) Error() string {
	return e.Message
}

func crud(r rune) bool {
	return r <= 32 ||
		r == ',' ||
		r == ':' ||
		r == ';' ||
		r == '<' ||
		r == '>' ||
		r == '"' ||
		r == '\\' ||
		r == '\''
}

/*
 * Copy over a string to the destination, but avoid special
 * characters ('\n', '<' and '>') and remove crud at the end
 */

func stringNoCRUD(s string) string {
	s = strings.TrimLeftFunc(s, crud)
	s = strings.TrimRightFunc(s, crud)
	var b strings.Builder
	b.Grow(len(s))
	for _, c := range s {
		if c == '\n' || c == '<' || c == '>' {
			continue
		}
		_, _ = b.WriteRune(c)
	}
	return b.String()
}

type Content struct {
	Text     string
	Hash     string
	Mode     filemode.FileMode
	IsBinary bool
}

func ReadContent(p string, textconv bool) (*Content, error) {
	fd, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	si, err := fd.Stat()
	if err != nil {
		return nil, err
	}
	h := plumbing.NewHasher()
	if _, err := io.Copy(h, fd); err != nil {
		return nil, err
	}
	fc := &Content{
		Hash: h.Sum().String(),
	}

	if fc.Mode, err = filemode.NewFromOS(si.Mode()); err != nil {
		return nil, err
	}
	if si.Size() > diferenco.MAX_DIFF_SIZE {
		fc.IsBinary = true
		return fc, nil
	}
	if _, err := fd.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	if fc.Text, _, err = diferenco.ReadUnifiedText(fd, si.Size(), textconv); err != nil {
		if err == diferenco.ErrNonTextContent {
			fc.IsBinary = true
			return fc, nil
		}
		return nil, err
	}
	return fc, nil
}

func ReadText(p string, textconv bool) (string, error) {
	fd, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer fd.Close()
	si, err := fd.Stat()
	if err != nil {
		return "", err
	}
	content, _, err := diferenco.ReadUnifiedText(fd, si.Size(), textconv)
	return content, err
}

type NopWriteCloser struct {
	io.Writer
}

func (NopWriteCloser) Close() error {
	return nil
}
