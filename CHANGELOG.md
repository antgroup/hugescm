# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.28.0] - 2026-06-09

### Fixed

- **term**: Fix OSC 11 background color query leaking `11;rgb:ffff/ffff/ffff` on terminals that do not support the protocol (e.g., web-based IDEs, basic xterm). Replace `lipgloss/v2/compat` package-level eager query with a lazy `sync.Once` detection that only fires when `detectColorLevel() >= Level16M` or `VTE_VERSION` is set
- **patchview**: Inline `hasDarkBackground()` wrapper into `DefaultStyle()`

### Changed

- **term**: Add `HasDarkBackground()` (lazy, safe) and `AdaptiveColor` type to `modules/term`, removing all imports of `charm.land/lipgloss/v2/compat`
- **term**: Export `MakeRaw` / `Restore` wrappers so callers no longer need a direct `golang.org/x/term` dependency

## [0.27.0] - 2026-06-08

### Fixed

- **config**: Fix `StringArray` TOML decoding failure when value is a single string (e.g., `extraHeader = "X-Custom: value"`) — replace dead `UnmarshalTOML(any)` (BurntSushi/toml interface) with `UnmarshalText` for `pelletier/go-toml/v2` compatibility
- **deflect**: Exclude tmp pack files from `Auditor.Size()` calculation; surface garbage stats in `hot stat/az`

## [0.26.0] - 2026-06-05

### Changed

- **patchview**: Flatten `DiffViewStyle` + `PatchViewStyle` into a single `Style` struct; add `Symbol` field to `LineStyle` for independent `+`/`-` symbol styling
- **patchview**: Separate symbol rendering from code content so symbols stay visible during horizontal scroll; remove diff percentage indicator
- **patchview**: Fix syntax highlight background color lost for zero-value tokens

### Fixed

- **hot replay**: Add case-insensitive ref conflict detection for Windows/macOS — `detectCaseConflicts()` checks for refs that differ only in case and reports them before `update-ref` fails
- **hot replay**: Improve `RefUpdater.closeWithError` to preserve both stderr and close error context instead of discarding one
- **hot replay**: Remove `pack.packSizeLimit=16g` from `git gc` invocation
- **setup**: Include `hot.exe` in Windows installer (`zeta.iss`)

### Documentation

- **CDC**: Major revision (v2.0 → v3.0) — fix mask strategy naming, update default parameters, add tail-merge and call-chain sections, correct config file paths
- **config**: Fix Size format description (unified 1024-based units), correct system config path
- **hot**: Document `hot diff` and `hot show` commands
- **README**: Add `status-perf-large-file.md` link; fix fragment config example

## [0.25.0] - 2026-06-03

### Added

- **Image Rendering**: `zeta cat` can now render images inline on capable terminals (`--image` flag)
- **`hot stat` Dashboard**: Redesign `hot stat` as a lipgloss card-style dashboard
- **Hexview**: Use rounded corners for border styling

### Changed

- **diferenco**: Refactor text/merge API and fix charset parsing; remove experimental parallel merge code (~2100 lines removed)
- **patchview**: Replace deprecated `charmtone.Charcoal` with `Char`
- **Translations**: Add zh-CN translation for `cat --image` flag

### Fixed

- **push**: Fix critical regression where all push operations fail with "read object length error: EOF" — caused by `unconvert tidy` (`4d8f045b`) incorrectly reusing the `size` variable for both wire-length header and payload verification
- **push**: Propagate write errors via `pipeWriter.CloseWithError` instead of silently closing the pipe
- **ssh**: Fix host alias lookup, swallowed IO error, and known_hosts test

## [0.24.0] - 2026-05-29

### Added

- **Configuration System Rewrite**: New TOML-based configuration system in `modules/zeta/config`
  - Introduce `Value`, `Document` abstractions with strong typing and validation
  - Add `codec_toml.go` for round-trip TOML encoding/decoding
  - Add comprehensive compatibility tests against legacy behavior
  - Replace `github.com/BurntSushi/toml` with `github.com/pelletier/go-toml/v2`
- **`pathmatch` Module**: New Git-compatible path matching module (`modules/pathmatch`)
  - Idiomatic Go port of Git's `wildmatch.c` (v2.54.0) with full bracket / POSIX class support
  - Case-fold and no-case-fold build variants
  - 1128 lines of test cases and 232 lines of fix-regression tests
  - Replaces and removes the legacy `modules/wildmatch` module
- **`--json` Output Flag**: Add structured JSON output across commands
  - `zeta tag`, `zeta branch`, `zeta config`, `zeta diff`, `zeta stash`, `zeta status`, `zeta log`, `zeta remote`, `zeta version`
- **Hot Subcommand JSON / Raw / Dry-run**:
  - `hot show`: support tree/blob objects with structured JSON output
  - `hot scan-refs` / `hot expire-refs`: add `--json`, `--raw` and `--dry-run`
- **PatchView Enhancements**:
  - Support `Enter` / `Space` / `PgUp` / `PgDown` for diff scrolling
  - Borderless key:value top header with full hash and colorized files
  - Status bar rounded border with optional top header card
  - Add layout contract tests and `sanitizeLine` for safer rendering
- **Worktree Status Cache**: New cache for `worktree_status` to speed up `status` on repos with large files
  - Add `modules/merkletrie/filesystem/cache.go` and supporting tests
  - Add `docs/status-perf-large-file.md`

### Changed

- **Go Version**: Bump to Go 1.26.3
- **Signal Handling**: Migrate `zeta` and `hot` commands to `signal.NotifyContext` for cleaner cancellation
- **CDC Refactor**: Inline chunk strategy into `writeFixedFragments` / `writeCDCFragments`; split out `cdc_gear.go`; add `cdc_test.go`
- **Safetensors**: Restructure parsing / writing paths alongside the CDC refactor
- **Ignore Engine**: Rewrite `modules/plumbing/format/ignore/pattern.go` to use the new `pathmatch`-style segment matcher, removing the `path/filepath` dependency
- **PatchView Cache**: Switch syntax highlight cache from custom LRU to `hashicorp/golang-lru/v2`
- **Diff Parser**: Remove deprecated `Patch.Binary` field and harden the parser (`cmd/hot/pkg/diff`)
- **Progress Bar**: Rewrite/clean up `modules/progressbar` (~570 lines of churn); fix progress bar / cancellation behavior
- **Lint / Code Quality**:
  - `error.As` cleanup across the codebase
  - `staticcheck QF1008` fixes
  - `unconvert`, `golangci-lint`, `go fix` cleanups; add `.golangci.yml`
  - `bufio.NewScanner` warning cleanup
- **Translations**: Add zh-CN strings for new flags / messages; drop legacy `zh-CN.tomlp1`

### Fixed

- Fix `hot diff` command
- Fix `hot` git version detection
- Fix `show` to render merge-commit metadata without entering the pager TUI
- Fix `branch` / `log`: restore pager in `ListBranch` and remove default JSON limit in `log`
- Fix `findAllBackends` bug
- Fix patchview regression after `zeta show --nav` revert
- `keyring`: fix file storage lock ordering and stale-lock detection
- `systemproxy`: harden `scutil` parsing on macOS; fix Windows SOCKS URL and `CONNECT 407` error reporting
- Bypass an out-of-bounds access bug in `go fix`

### Dependencies

- **Updated**:
  - `github.com/alecthomas/chroma/v2` v2.23.1 → v2.26.1
  - `github.com/charmbracelet/x/exp/charmtone` → 2026-05-27 snapshot
  - `github.com/charmbracelet/ultraviolet` → 2026-05-25 snapshot
  - `github.com/charmbracelet/x/exp/slice` → 2026-05-27 snapshot
  - `github.com/ebitengine/purego` v0.10.0 → v0.10.1
  - `github.com/go-sql-driver/mysql` v1.9.3 → v1.10.0
  - `github.com/klauspost/compress` v1.18.5 → v1.18.6
  - `github.com/dlclark/regexp2` v1.12.0 → `regexp2/v2` v2.1.1
  - `golang.org/x/crypto` v0.50.0 → v0.52.0
  - `golang.org/x/net` v0.53.0 → v0.55.0
  - `golang.org/x/sys` v0.43.0 → v0.45.0
  - `golang.org/x/term` v0.42.0 → v0.43.0
  - `golang.org/x/text` v0.36.0 → v0.37.0
- **Added**:
  - `github.com/hashicorp/golang-lru/v2` v2.0.7
  - `github.com/pelletier/go-toml/v2` v2.3.1
- **Removed**:
  - `github.com/BurntSushi/toml`
  - `modules/wildmatch` (replaced by `modules/pathmatch`)

## [0.23.0] - 2026-04-22

### Added

- **Hot Diff/Show Commands**: Add `hot diff` and `hot show` commands for viewing differences in git repositories
- **Interactive Diff Navigation**: Add `--nav` flag to `zeta diff` and `zeta show` commands for built-in interactive diff viewer with syntax highlighting
- **Advanced Viewport Module**: Import feature-rich viewport component with text wrapping, selection, and filtering capabilities
- **MultiBar Progress**: Rewrite progress bar component using `bubbles/progress` with concurrent multi-bar rendering and EWMA speed tracking
- **LOONG64 Support**: Enable builds for LoongArch64 architecture

### Changed

- **Patch View Improvements**:
  - Refactor patchview module with improved navigation mode
  - Add LRU cache for syntax highlighting (up to 1000 entries)
  - Remove standalone word-diff in favor of integrated nav mode
  - Enhance diff theme and rendering
- **TUI Enhancements**:
  - Switch to custom viewport implementation for better control
  - Optimize pager rendering performance
  - Improve word diff performance
- **Code Cleanup**:
  - Remove legacy `diffformat.go` module (287 lines removed)
  - Code tidy and refactoring across multiple modules

### Fixed

- Fix double close issue in `writeCredentials` for keyring file storage
- Harden keyring file storage with atomic writes and lock handling
- Fix `truncatePath` in hot commands
- Fix pager status bar space display
- Fix multi `-m` flag handling in commit command
- Fix small bug in diferenco module

### Dependencies

- **Updated**:
  - `charm.land/bubbletea/v2` from v2.0.2 to v2.0.6
  - `charm.land/lipgloss/v2` from v2.0.2 to v2.0.3
  - `golang.org/x/crypto` from v0.49.0 to v0.50.0
  - `golang.org/x/net` from v0.52.0 to v0.53.0
  - `golang.org/x/sys` from v0.42.0 to v0.43.0
  - `golang.org/x/term` from v0.41.0 to v0.42.0
  - `golang.org/x/text` from v0.35.0 to v0.36.0
- **Added**: `github.com/zeebo/xxh3` v1.1.0 for fast hashing
- **Removed**: `github.com/vbauerster/mpb/v8` (replaced by custom MultiBar implementation)

## [0.22.0] - 2026-03-27

### Added

- **FastCDC Chunking**: Implement FastCDC (Content-Defined Chunking) algorithm for AI model storage optimization, supporting Safetensors format (`#7`)
- **Word Diff**: Support simple word-level diff in `zeta diff` and `zeta show` commands
- **Secure Keyring Storage**: Add keyring support for secure credential storage
  - macOS: Keychain integration
  - Windows: Windows Credential Manager integration
  - Linux: File-based storage backend
- **Network Filesystem Warning**: Automatically detect and warn about network filesystems (NFS, Ceph, SMB) with highlighted filesystem names

### Changed

- **TUI Framework Migration**: Switch from custom survey module to `charmbracelet/huh` for better terminal UI experience (removed 10,000+ lines of legacy code)
- **Improved Table Rendering**: Replace `go-pretty` with `bubbletea table` for better TUI rendering in `zeta hot` commands
- **Enhanced Pager**: Add space key support for page navigation in TUI pager
- **Diferenco Improvements**:
  - Add `name` field to `FileStat`
  - Add `Format()` method to `Patch`
  - Optimize `MergeParallel` implementation
  - Improve `SplitWords` algorithm
  - Enhance Myers diff algorithm
- **Performance Optimizations**:
  - Optimize worktree operations
  - Improve commit decoding efficiency
  - Enhance system proxy detection accuracy

### Fixed

- Fix multiple keyring issues on Windows and Unix platforms
- Fix panic in `wildmatch` pattern matching
- Fix tree cache corruption issues
- Fix missing context in commit walker
- Fix zlib handling edge cases
- Fix split words boundary issues
- Fix trace color display

### Dependencies

- **Go 1.26**: Upgrade to Go 1.26.0
- **Removed**: `testify` testing dependency
- **Updated**:
  - `charm.land` ecosystem modules (bubbles, bubbletea, glamour, huh, lipgloss)
  - `github.com/ProtonMail/go-crypto` v1.4.1
  - `github.com/klauspost/compress` v1.18.5
  - `github.com/dgraph-io/ristretto/v2` v2.4.0
  - Multiple `golang.org/x` modules

### Documentation

- Add CDC (Content-Defined Chunking) documentation (`docs/cdc.md`)
- Update README with latest features
- Improve documentation organization

### Internationalization

- Complete Chinese (zh-CN) translations
- Add missing i18n entries

## [0.21.0] - 2025-12-16

### Added

- Initial stable release with core version control features
- Metadata and file data separation architecture
- Distributed database for metadata storage
- Object storage for file content
- Efficient transfer protocol
- Fragment object support for large files
- Support for AI model development, game development, and monorepo scenarios

[Unreleased]: https://code.alipay.com/zeta/zeta/compare/v0.28.0...HEAD
[0.28.0]: https://code.alipay.com/zeta/zeta/compare/v0.27.0...v0.28.0
[0.27.0]: https://code.alipay.com/zeta/zeta/compare/v0.26.0...v0.27.0
[0.26.0]: https://code.alipay.com/zeta/zeta/compare/v0.25.0...v0.26.0
[0.25.0]: https://code.alipay.com/zeta/zeta/compare/v0.24.0...v0.25.0
[0.24.0]: https://code.alipay.com/zeta/zeta/compare/v0.23.0...v0.24.0
[0.23.0]: https://code.alipay.com/zeta/zeta/compare/v0.22.0...v0.23.0
[0.22.0]: https://code.alipay.com/zeta/zeta/compare/v0.21.0...v0.22.0