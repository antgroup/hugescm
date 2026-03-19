//go:build darwin

package keyring

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const (
	storageSecurity = "security" // macOS only: /usr/bin/security CLI
	// securityCLIPath is the path to the security command-line tool
	securityCLIPath = "/usr/bin/security"

	// securityErrNotFoundExitCode is the exit code returned by security CLI when an item is not found.
	securityErrNotFoundExitCode = 44

	// maxSecurityCommandLen is an internal defensive limit for security CLI commands.
	// This is NOT a documented limit of the security CLI itself, but rather a sanity check
	// to prevent unreasonably large credentials that may indicate a problem upstream.
	maxSecurityCommandLen = 64 * 1024
)

var (
	shellEscapePattern = regexp.MustCompile(`[^\w@%+=:,./-]`)
)

// protocolFourCC converts a protocol string to the 4-character code used by
// macOS security CLI's -r flag. These codes correspond to the kSecAttrProtocol
// constants in Security.framework (e.g., kSecAttrProtocolHTTPS = 'htps').
// Returns empty string for unknown protocols, in which case the caller should
// omit the -r flag to avoid incorrect matching.
func protocolFourCC(protocol string) string {
	switch strings.ToLower(protocol) {
	case "http":
		return "http"
	case "https":
		return "htps"
	case "ftp":
		return "ftp "
	case "ftps":
		return "ftps"
	case "imap":
		return "imap"
	case "imaps":
		return "imps"
	case "smtp":
		return "smtp"
	default:
		return ""
	}
}

// isSecurityNotFoundError checks if the error indicates that the item was not found.
// It prioritizes exit code 44, with string matching as a fallback for compatibility.
func isSecurityNotFoundError(err error, output []byte) bool {
	// Priority 1: Check exit code 44 (official not-found indicator)
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		if exitErr.ExitCode() == securityErrNotFoundExitCode {
			return true
		}
	}
	// Priority 2: Fallback to string matching for compatibility
	outputStr := string(output)
	return strings.Contains(outputStr, "could not be found") ||
		strings.Contains(outputStr, "The specified item could not be found")
}

// shellQuote returns a shell-escaped version of the string s.
// The returned value is a string that can safely be used as one token in a shell command line.
//
// NOTE: This quoting logic is specifically designed for the `security -i` interactive mode,
// which has its own command parser. The behavior is based on empirical testing of security CLI
// and is NOT guaranteed by Apple documentation. This implementation may need adjustment if
// future macOS versions change the CLI parser behavior.
func shellQuote(s string) string {
	if len(s) == 0 {
		return "''"
	}
	if shellEscapePattern.MatchString(s) {
		return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
	}
	return s
}

// getFromSecurityCLI retrieves credentials using /usr/bin/security CLI.
// Uses find-internet-password which is compatible with git-credential-osxkeychain.
// This is a fallback when Security.framework access is blocked by security software.
// The query parameters must match the purego implementation in keyring_darwin.go.
func getFromSecurityCLI(ctx context.Context, cred *Cred) (*Cred, error) {
	if cred == nil {
		return nil, errors.New("credential cannot be nil")
	}

	if cred.Server == "" {
		return nil, errors.New("server is required")
	}

	// Use security find-internet-password to retrieve credentials
	// This matches the purego implementation and git-credential-osxkeychain pattern
	// -s: server name (host only, not full URL)
	// -r: protocol (4-char code, e.g., htps for https)
	// -P: port (optional)
	// -p: path (optional)
	// -a: account name (optional, but improves precision when multiple accounts exist)
	// -g: display password
	args := []string{"find-internet-password", "-s", cred.Server}

	// Add protocol if known (matches purego kSecAttrProtocol)
	if fourCC := protocolFourCC(cred.Protocol); fourCC != "" {
		args = append(args, "-r", fourCC)
	}

	// Add port if specified (matches purego kSecAttrPort)
	if cred.Port != 0 {
		args = append(args, "-P", strconv.Itoa(cred.Port))
	}

	// Add path if specified (matches purego kSecAttrPath)
	if cred.Path != "" {
		args = append(args, "-p", cred.Path)
	}

	// Add account name for more precise query if available
	if cred.UserName != "" {
		args = append(args, "-a", cred.UserName)
	}

	// Add -g to display password
	args = append(args, "-g")

	cmd := exec.CommandContext(ctx, securityCLIPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if isSecurityNotFoundError(err, out) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("security find-internet-password failed: %w, output: %s", err, string(out))
	}

	return parseKeychainOutput(bytes.NewReader(out))
}

// parseKeychainOutput parses the output from security find-internet-password.
// Output format example:
//
//	keychain: "/Users/**/Library/Keychains/login.keychain-db"
//	version: 512
//	class: "inet"
//	attributes:
//	    "acct"<blob>="username"
//	    "acct"<blob>=0x75736572  (hex format on some macOS versions)
//	    "srvr"<blob>="https://zeta.example.io"
//	password: "password"
//	password: 0x68656c6c6f  (hex format on some macOS versions)
func parseKeychainOutput(r io.Reader) (*Cred, error) {
	scanner := bufio.NewScanner(r)
	cred := &Cred{}
	var err error

	for scanner.Scan() {
		line := strings.TrimFunc(scanner.Text(), unicode.IsSpace)

		// Parse account name: "acct"<blob>="username" or "acct"<blob>=0x...
		if suffix, ok := strings.CutPrefix(line, `"acct"`); ok {
			_, acct, _ := strings.Cut(suffix, "=")
			acct = strings.TrimFunc(acct, unicode.IsSpace)
			cred.UserName, err = parseBlobValue(acct)
			if err != nil {
				// If parsing fails, try using it as-is (be lenient for CLI fallback)
				cred.UserName = acct
			}
			continue
		}

		// Parse password: password: "password" or password: 0x...
		if password, ok := strings.CutPrefix(line, "password:"); ok {
			password = strings.TrimFunc(password, unicode.IsSpace)
			cred.Password, err = parseBlobValue(password)
			if err != nil {
				// If parsing fails, try using it as-is (be lenient for CLI fallback)
				cred.Password = password
			}
			continue
		}
	}

	// Check for scanner errors (e.g., line too long)
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse keychain output: %w", err)
	}

	// Validate that password was parsed successfully
	// Password is the core field - without it, the credential is incomplete
	if cred.Password == "" {
		return nil, ErrNotFound
	}

	return cred, nil
}

// parseBlobValue parses a value from security CLI output.
// It handles both quoted strings ("value") and hex format (0x68656c6c6f).
func parseBlobValue(s string) (string, error) {
	// Handle hex format: 0x68656c6c6f
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		decoded, err := hex.DecodeString(s[2:])
		if err != nil {
			return "", fmt.Errorf("failed to decode hex value: %w", err)
		}
		return string(decoded), nil
	}

	// Handle quoted string
	return strconv.Unquote(s)
}

// storeToSecurityCLI stores credentials using /usr/bin/security CLI.
// Uses add-internet-password which is compatible with git-credential-osxkeychain.
// The storage parameters must match the purego implementation in keyring_darwin.go.
func storeToSecurityCLI(ctx context.Context, cred *Cred) error {
	if cred == nil {
		return errors.New("credential cannot be nil")
	}

	if cred.UserName == "" {
		return errors.New("username cannot be empty")
	}
	if cred.Password == "" {
		return errors.New("password cannot be empty")
	}
	if cred.Server == "" {
		return errors.New("server cannot be empty")
	}

	// Use security -i for interactive mode to handle special characters
	cmd := exec.CommandContext(ctx, securityCLIPath, "-i")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	if err = cmd.Start(); err != nil {
		return fmt.Errorf("failed to start security command: %w", err)
	}

	// Build the add-internet-password command
	// -U flag updates existing item if present
	// -s: server name (host only, not full URL) - matches purego kSecAttrServer
	// -r: protocol (4-char code) - matches purego kSecAttrProtocol
	// -P: port (optional) - matches purego kSecAttrPort
	// -p: path (optional) - matches purego kSecAttrPath
	// -a: account name - matches purego kSecAttrAccount
	// -w: password
	var commandBuilder strings.Builder
	commandBuilder.WriteString("add-internet-password -U -s ")
	commandBuilder.WriteString(shellQuote(cred.Server))

	// Add protocol if known (matches purego kSecAttrProtocol)
	if fourCC := protocolFourCC(cred.Protocol); fourCC != "" {
		commandBuilder.WriteString(" -r ")
		commandBuilder.WriteString(fourCC)
	}

	if cred.Port != 0 {
		commandBuilder.WriteString(" -P ")
		commandBuilder.WriteString(strconv.Itoa(cred.Port))
	}

	if cred.Path != "" {
		commandBuilder.WriteString(" -p ")
		commandBuilder.WriteString(shellQuote(cred.Path))
	}

	commandBuilder.WriteString(" -a ")
	commandBuilder.WriteString(shellQuote(cred.UserName))
	commandBuilder.WriteString(" -w ")
	commandBuilder.WriteString(shellQuote(cred.Password))
	commandBuilder.WriteString("\n")

	command := commandBuilder.String()

	// Limit command length as a defensive measure against unreasonably large input.
	// Keychain itself doesn't have this limit, but extremely long server names or
	// passwords usually indicate a problem upstream. This limit is conservative
	// and can be increased if needed.
	if len(command) > maxSecurityCommandLen {
		_ = stdin.Close()
		_ = cmd.Wait()
		return ErrSetDataTooBig
	}

	// Write the command
	if _, err := io.WriteString(stdin, command); err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return fmt.Errorf("failed to write command: %w", err)
	}

	// Close stdin to signal end of input
	if err = stdin.Close(); err != nil {
		_ = cmd.Wait()
		return fmt.Errorf("failed to close stdin: %w", err)
	}

	// Wait for the command to complete
	if err = cmd.Wait(); err != nil {
		return fmt.Errorf("security add-internet-password failed: %w", err)
	}

	return nil
}

// eraseFromSecurityCLI removes credentials using /usr/bin/security CLI.
// Uses delete-internet-password to match the find-internet-password pattern.
// The query parameters must match the purego implementation in keyring_darwin.go.
func eraseFromSecurityCLI(ctx context.Context, cred *Cred) error {
	if cred == nil {
		return errors.New("credential cannot be nil")
	}

	if cred.Server == "" {
		return errors.New("server is required")
	}

	// Use delete-internet-password to match find-internet-password
	// -s: server name (host only, not full URL) - matches purego kSecAttrServer
	// -r: protocol (4-char code) - matches purego kSecAttrProtocol
	// -P: port (optional) - matches purego kSecAttrPort
	// -p: path (optional) - matches purego kSecAttrPath
	// -a: account name (optional, but ensures precise deletion when multiple accounts exist)
	args := []string{"delete-internet-password", "-s", cred.Server}

	// Add protocol if known (matches purego kSecAttrProtocol)
	if fourCC := protocolFourCC(cred.Protocol); fourCC != "" {
		args = append(args, "-r", fourCC)
	}

	// Add port if specified (matches purego kSecAttrPort)
	if cred.Port != 0 {
		args = append(args, "-P", strconv.Itoa(cred.Port))
	}

	// Add path if specified (matches purego kSecAttrPath)
	if cred.Path != "" {
		args = append(args, "-p", cred.Path)
	}

	// Add account name for more precise deletion if available
	if cred.UserName != "" {
		args = append(args, "-a", cred.UserName)
	}

	cmd := exec.CommandContext(ctx, securityCLIPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Item not found is not an error - deletion is idempotent
		if isSecurityNotFoundError(err, out) {
			return nil
		}
		return fmt.Errorf("security delete-internet-password failed: %w, output: %s", err, string(out))
	}

	return nil
}
