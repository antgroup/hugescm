package deflect

import (
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/strengthen"
)

// Typical .git/config format:
// [core]
// 	repositoryformatversion = 1
// 	filemode = true
// 	bare = false
// 	logallrefupdates = true
// 	ignorecase = true
// 	precomposeunicode = true
// [extensions]
// 	objectformat = sha256

const (
	// DefaultFileSizeLimit is the default file size threshold (50 MiB) for identifying large files
	DefaultFileSizeLimit = strengthen.MiByte * 50
	// hugeSizeLimit defines the threshold (15 MiB) for considering files as "huge" for statistics
	hugeSizeLimit = strengthen.MiByte * 15
)

// Option configures the auditing behavior for repository analysis
type Option struct {
	// Limit is the file size threshold in bytes. Files larger than this will be rejected.
	Limit int64
	// OnOversized is a callback function called for each file that exceeds the limit.
	// Returns an error to stop processing, or nil to continue.
	OnOversized func(oid string, size int64) error
	// QuarantineMode enables analysis of incoming objects in Git quarantine mode.
	// When enabled, analyzes both the main repository and quarantine directory.
	QuarantineMode bool
}

// pack represents a Git pack file with its path and size
type pack struct {
	path string // Full path to the .pack file
	size int64  // Size of the pack file in bytes
}

// Auditor is the main analyzer for Git repository large file detection
type Auditor struct {
	*Option         // Embedded auditing configuration
	repoPath string // Path to the Git repository root directory
	size     int64  // Total size of all objects in bytes
	delta    int64  // Size increment for quarantine mode analysis
	hugeSum  int64  // Total size of files exceeding hugeSizeLimit
	rawsz    int64  // Size of hash values (20 for SHA1, 32 for SHA256)
	counts   uint32 // Total number of objects analyzed
	packs    []pack // List of pack files to be analyzed
	tmpPacks uint32 // Count of temporary pack files (tmp_*.pack)
}

// NewAuditor creates a new Auditor instance for analyzing a Git repository
// Parameters:
//   - repoPath: path to the Git repository directory
//   - shaFormat: the hash format (SHA1 or SHA256) used by the repository
//   - opts: optional filtering configuration (nil for defaults)
func NewAuditor(repoPath string, shaFormat git.HashFormat, opts *Option) *Auditor {
	au := &Auditor{
		repoPath: repoPath,
		rawsz:    int64(shaFormat.RawSize()),
	}
	if opts == nil {
		au.Option = &Option{
			Limit: DefaultFileSizeLimit,
		}
		return au
	}
	au.Option = &Option{
		Limit:          opts.Limit,
		OnOversized:    opts.OnOversized,
		QuarantineMode: opts.QuarantineMode,
	}
	if au.Limit <= 0 {
		au.Limit = DefaultFileSizeLimit // avoid --> au.Limit <= 0
	}
	return au
}

// HashLen returns the hash length in bytes (20 for SHA1, 32 for SHA256)
func (a *Auditor) HashLen() int64 {
	return a.rawsz
}

// Counts returns the total number of objects analyzed
func (a *Auditor) Counts() uint32 {
	return a.counts
}

// Size returns the total size of all objects in bytes
func (a *Auditor) Size() int64 {
	return a.size
}

// Delta returns the size increment for quarantine mode analysis
func (a *Auditor) Delta() int64 {
	return a.delta
}

// HugeSUM returns the total size of files exceeding hugeSizeLimit
func (a *Auditor) HugeSUM() int64 {
	return a.hugeSum
}

// Execute performs the complete repository analysis:
// 1. Analyzes disk usage of loose objects and pack files
// 2. Calls the SizeReceiver callback with total size if provided
// 3. Analyzes each pack file for large objects
func (a *Auditor) Execute() error {
	if err := a.Du(); err != nil {
		return err
	}
	for _, p := range a.packs {
		if err := a.analyzePack(&p); err != nil {
			return err
		}
	}
	return nil
}

// onOversized handles rejected large files by calling the configured Rejector or printing to stderr
func (a *Auditor) onOversized(oid string, size int64) error {
	if a.OnOversized == nil {
		fmt.Fprintf(os.Stderr, "blob: %s compressed size: %s\n", oid, strengthen.FormatSize(size))
		return nil
	}
	return a.OnOversized(oid, size)
}

// Du is a convenience function that calculates the total disk usage of a Git repository
// Returns the total size in bytes and any error encountered
func Du(repoPath string) (int64, error) {
	shaFormat, err := git.HashFormatResult(repoPath)
	if err != nil {
		return 0, err
	}
	au := NewAuditor(repoPath, shaFormat, &Option{
		Limit:          strengthen.GiByte,
		QuarantineMode: false,
		OnOversized: func(oid string, size int64) error {
			return nil
		},
	})
	if err := au.Du(); err != nil {
		return 0, err
	}
	return au.Size(), nil
}
