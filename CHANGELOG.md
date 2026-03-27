# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/antgroup/hugescm/compare/v0.22.0...HEAD
[0.22.0]: https://github.com/antgroup/hugescm/compare/v0.21.0...v0.22.0
[0.21.0]: https://github.com/antgroup/hugescm/releases/tag/v0.21.0