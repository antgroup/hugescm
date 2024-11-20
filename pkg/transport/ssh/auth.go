// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/antgroup/hugescm/modules/env"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/survey"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

func keyTypeName(key ssh.PublicKey) string {
	kt := key.Type()
	switch kt {
	case "ssh-rsa":
		return "RSA"
	case "ssh-dss":
		return "DSA"
	case "ssh-ed25519":
		return "ED25519"
	default:
		if strings.HasPrefix(kt, "ecdsa-sha2-") {
			return "ECDSA"
		}
	}
	return kt
}

// DefaultKnownHostsPath returns default user knows hosts file.
func DefaultKnownHostsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "known_hosts"), nil
}

// DefaultKnownHosts returns host key callback from default known hosts path, and error if any.
func DefaultKnownHosts() (ssh.HostKeyCallback, error) {
	p, err := DefaultKnownHostsPath()
	if err != nil {
		return nil, err
	}
	return knownhosts.New(p)
}

func (c *client) readPassword() (string, error) {
	if !env.ZETA_TERMINAL_PROMPT.SimpleAtob(true) {
		return "", errors.New("terminal prompts disabled")
	}
	var password string
	prompt := &survey.Password{
		Message: fmt.Sprintf("Password for '%s@%s':", c.User, c.Endpoint),
	}
	if err := survey.AskOne(prompt, &password); err != nil {
		return "", err
	}
	return "", nil
}

func addForKnownHost(host string, remote net.Addr, key ssh.PublicKey, knownHostsFile string) error {
	fd, err := os.OpenFile(knownHostsFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}

	defer fd.Close()
	remoteNormalized := knownhosts.Normalize(remote.String())
	hostNormalized := knownhosts.Normalize(host)
	addresses := []string{remoteNormalized}

	if hostNormalized != remoteNormalized {
		addresses = append(addresses, hostNormalized)
	}
	// default:
	// "The authenticity of host '%s (%s)' can't be established.\n%s key fingerprint is %s\nAre you sure you want to continue connecting (yes/no)? "
	_, err = fd.WriteString(knownhosts.Line(addresses, key) + "\n")

	return err
}

func unfoldKeyError(hostname string, key ssh.PublicKey, ke *knownhosts.KeyError) {
	k0 := ke.Want[0]
	hostKeyType := keyTypeName(key)
	localKeyType := keyTypeName(k0.Key)
	fmt.Fprintf(os.Stderr, `@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
@    WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!     @
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
IT IS POSSIBLE THAT SOMEONE IS DOING SOMETHING NASTY!
Someone could be eavesdropping on you right now (man-in-the-middle attack)!
It is also possible that the %s host key has just been changed.
The fingerprint for the %s key sent by the remote host is
%s.
Please contact your system administrator.
Add correct host key in %s to get rid of this message.
Offending key in %s:%d. The fingerprint is
%s.
%s host key for %s has changed and you have requested strict checking.
Host key verification failed.
`, "\x1b[33m"+localKeyType+"\x1b[0m",
		"\x1b[33m"+hostKeyType+"\x1b[0m",
		"\x1b[33m"+ssh.FingerprintSHA256(key)+"\x1b[0m",
		k0.Filename,
		k0.Filename,
		k0.Line,
		"\x1b[33m"+ssh.FingerprintSHA256(k0.Key)+"\x1b[0m",
		hostname, hostKeyType)
}

func checkForKnownHosts(host string, remote net.Addr, key ssh.PublicKey, knownHostsFile string) (bool, error) {
	callback, err := knownhosts.New(knownHostsFile)
	if err != nil {
		return false, err
	}
	if err = callback(host, remote, key); err == nil {
		return true, nil
	}
	var keyErr *knownhosts.KeyError
	if errors.As(err, &keyErr) && len(keyErr.Want) > 0 {
		unfoldKeyError(host, key, keyErr)
		return true, keyErr
	}
	return false, err
}

func (c *client) HostKeyCallback(host string, remote net.Addr, key ssh.PublicKey) error {
	knownHostsFile, err := DefaultKnownHostsPath()
	if err != nil {
		return err
	}
	found, err := checkForKnownHosts(host, remote, key, knownHostsFile)
	if found {
		return err
	}
	return addForKnownHost(host, remote, key, knownHostsFile)
}

func (c *client) openPrivateKey(name string) (ssh.Signer, error) {
	fd, err := os.Open(name)
	if err != nil {
		c.DbgPrint("read private key %s error: %v", name, err)
		return nil, err
	}
	defer fd.Close()
	buf, err := io.ReadAll(fd)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(buf)
	if err != nil {
		return nil, err
	}
	pk := signer.PublicKey()
	c.DbgPrint("Offering public key: %s %s", name, ssh.FingerprintSHA256(pk))
	return signer, nil
}

func (c *client) sshAuthSigners() ([]ssh.Signer, error) {
	if strengthen.SimpleAtob("ZETA_NO_SSH_AUTH_SOCK", false) {
		return nil, nil
	}
	sock, ok := os.LookupEnv("SSH_AUTH_SOCK")
	if !ok {
		return nil, nil
	}
	sshAgentConn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("could not find ssh agent: %w", err)
	}
	defer sshAgentConn.Close()
	cc := agent.NewClient(sshAgentConn)
	return cc.Signers()
}

func (c *client) PublicKeys() ([]ssh.Signer, error) {
	homePath, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	signers := make([]ssh.Signer, 0, 5)
	// TODO: support id_ed25519_sk id_ecdsa_sk ??
	for _, n := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
		keyPath := filepath.Join(homePath, ".ssh", n)
		signer, err := c.openPrivateKey(keyPath)
		if err != nil {
			continue
		}
		signers = append(signers, signer)
	}
	if agnetSigners, err := c.sshAuthSigners(); err == nil && len(agnetSigners) > 0 {
		signers = append(signers, agnetSigners...)
	}
	return signers, nil
}
