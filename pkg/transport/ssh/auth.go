// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/antgroup/hugescm/modules/env"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/survey"
	"github.com/antgroup/hugescm/modules/trace"
	"github.com/antgroup/hugescm/pkg/transport/ssh/knownhosts"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// NewKnownHostsCallback returns ssh.HostKeyCallback based on a file based on a
// known_hosts file. http://man.openbsd.org/sshd#SSH_KNOWN_HOSTS_FILE_FORMAT
//
// If list of files is empty, then it will be read from the SSH_KNOWN_HOSTS
// environment variable, example:
//
//	/home/foo/custom_known_hosts_file:/etc/custom_known/hosts_file
//
// If SSH_KNOWN_HOSTS is not set the following file locations will be used:
//
//	~/.ssh/known_hosts
//	/etc/ssh/ssh_known_hosts
func NewKnownHostsCallback(files ...string) (ssh.HostKeyCallback, error) {
	db, err := newKnownHostsDb(files...)
	if err != nil {
		return nil, err
	}
	return db.HostKeyCallback(), err
}

func newKnownHostsDb(files ...string) (*knownhosts.HostKeyDB, error) {
	var err error
	if len(files) == 0 {
		if files, err = getDefaultKnownHostsFiles(); err != nil {
			return nil, err
		}
	}

	if files, err = filterKnownHostsFiles(files...); err != nil {
		return nil, err
	}
	return knownhosts.NewDB(files...)
}

func getDefaultKnownHostsFiles() ([]string, error) {
	files := filepath.SplitList(os.Getenv("SSH_KNOWN_HOSTS"))
	if len(files) != 0 {
		return files, nil
	}

	homeDirPath, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	return []string{
		filepath.Join(homeDirPath, ".ssh/known_hosts"),
		"/etc/ssh/ssh_known_hosts",
	}, nil
}

func (c *client) readHostKeyDB() (err error) {
	c.hostKeyDB, err = newKnownHostsDb()
	return
}

func keyAlgoName(s string) string {
	if suffix, ok := strings.CutPrefix(s, "ssh-"); ok {
		return strings.ToUpper(suffix)
	}
	return s
}

// https://github.com/golang/go/issues/28870
func (c *client) HostKeyCallback(hostname string, remote net.Addr, key ssh.PublicKey) error {
	innerCallback := c.hostKeyDB.HostKeyCallback()
	trace.DbgPrint("Server host key: %s %s", key.Type(), ssh.FingerprintSHA256(key))
	err := innerCallback(hostname, remote, key)
	if !knownhosts.IsHostUnknown(err) {
		return err
	}
	homeDir, ferr := os.UserHomeDir()
	if ferr != nil {
		fmt.Fprintf(os.Stderr, "error: unable search user homeDir: %v", err)
		return err
	}
	fd, ferr := os.OpenFile(filepath.Join(homeDir, ".ssh/known_hosts"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if ferr != nil {
		fmt.Fprintf(os.Stderr, "error: unable open ~/.ssh/known_hosts: %v", ferr)
		return err
	}
	defer fd.Close() // nolint
	if ferr = knownhosts.WriteKnownHost(fd, hostname, remote, key); ferr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to add host %s to known_hosts: %v\n", hostname, err)
		return nil
	}
	serverName := hostname
	if domain, port, err := net.SplitHostPort(serverName); err == nil && port == "22" {
		serverName = domain
	}
	trace.DbgPrint("Permanently added '%s' (%s) to the list of known hosts", serverName, keyAlgoName(key.Type()))
	c.hostKeyDB = nil
	return nil
}

func filterKnownHostsFiles(files ...string) ([]string, error) {
	var out []string
	for _, file := range files {
		_, err := os.Stat(file)
		if err == nil {
			out = append(out, file)
			continue
		}

		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return out, nil
}

func (c *client) prepareAuthMethod() ([]ssh.AuthMethod, error) {
	auth := make([]ssh.AuthMethod, 0, 4)
	auth = append(auth, ssh.PublicKeysCallback(c.PublicKeys))
	if len(c.Password) != 0 {
		auth = append(auth, ssh.Password(c.Password)) // static password
		return auth, nil
	}
	if !env.ZETA_TERMINAL_PROMPT.SimpleAtob(true) {
		return auth, nil
	}
	auth = append(auth, ssh.PasswordCallback(func() (secret string, err error) {
		prompt := &survey.Password{
			Message: fmt.Sprintf("Password for '%s@%s':", c.User, c.Endpoint),
		}
		if err = survey.AskOne(prompt, &secret); err != nil {
			return
		}
		return
	}))
	return auth, nil
}

var (
	supportedHostKeys = sync.OnceValue(func() []string {
		keys := ssh.SupportedAlgorithms().HostKeys
		reorderedKeys := make([]string, 0, len(keys))
		reorderedKeys = append(reorderedKeys, ssh.KeyAlgoED25519)
		for _, k := range keys {
			if k != ssh.KeyAlgoED25519 {
				reorderedKeys = append(reorderedKeys, k)
			}
		}
		return reorderedKeys
	})
)

func (c *client) supportedHostKeyAlgos() []string {
	if hostKeyAlgorithms := c.hostKeyDB.HostKeyAlgorithms(net.JoinHostPort(c.Hostname, c.Port)); len(hostKeyAlgorithms) != 0 {
		return hostKeyAlgorithms
	}
	return supportedHostKeys()
}

func (c *client) openPrivateKey(name string) (ssh.Signer, error) {
	buf, err := os.ReadFile(name)
	if err != nil {
		trace.DbgPrint("read private key %s error: %v", name, err)
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(buf)
	if err != nil {
		return nil, err
	}
	pk := signer.PublicKey()
	trace.DbgPrint("Offering public key: %s %s", name, ssh.FingerprintSHA256(pk))
	return signer, nil
}

func (c *client) sshAuthSigners() ([]ssh.Signer, error) {
	if env.ZETA_NO_SSH_AUTH_SOCK.SimpleAtob(false) {
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
	defer sshAgentConn.Close() // nolint
	cc := agent.NewClient(sshAgentConn)
	return cc.Signers()
}

func (c *client) PublicKeys() ([]ssh.Signer, error) {
	if len(c.IdentityFile) != 0 {
		signer, err := c.openPrivateKey(strengthen.ExpandPath(c.IdentityFile))
		if err == nil {
			return []ssh.Signer{signer}, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	signers := make([]ssh.Signer, 0, 5)
	// TODO: support id_ed25519_sk id_ecdsa_sk ??
	for _, supportKey := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
		keyPath := filepath.Join(userHomeDir, ".ssh", supportKey)
		signer, err := c.openPrivateKey(keyPath)
		if err != nil {
			continue
		}
		signers = append(signers, signer)
	}
	if agentSigners, err := c.sshAuthSigners(); err == nil && len(agentSigners) > 0 {
		signers = append(signers, agentSigners...)
	}
	return signers, nil
}
