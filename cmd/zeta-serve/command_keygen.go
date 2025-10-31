// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

type Keygen struct {
	Type    string `name:"type" short:"t" help:"Generate private key type" default:"RSA"`
	BitSize int    `name:"bitSize" help:"Generates a random RSA private key of the given bit size" default:"2048"`
}

func (c *Keygen) genRAS() error {
	if c.BitSize < 1024 {
		c.BitSize = 2048
	}
	// Generate RSA key.
	key, err := rsa.GenerateKey(rand.Reader, c.BitSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GenKey error: %v\n", err)
		return err
	}

	// Encode private key to PKCS#1 ASN.1 PEM.
	keyPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		},
	)
	_, _ = fmt.Fprint(os.Stdout, string(keyPEM))
	return nil
}

func (c *Keygen) genED25519() error {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GenKey error: %v\n", err)
		return err
	}
	block, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "GenKey error: %v\n", err)
		return err
	}
	_, _ = fmt.Fprint(os.Stdout, string(pem.EncodeToMemory(block)))
	return nil
}

func (c *Keygen) genECDSA() error {
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GenKey error: %v\n", err)
		return err
	}
	block, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "GenKey error: %v\n", err)
		return err
	}
	_, _ = fmt.Fprint(os.Stdout, string(pem.EncodeToMemory(block)))
	return nil
}

func (c *Keygen) genX25519() error {
	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("GenKey error: %w", err)
	}
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("GenKeyError: %w", err)
	}
	_, _ = fmt.Fprint(os.Stdout, string(pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})))
	return nil
}

func (c *Keygen) Run(g *Globals) error {
	switch strings.ToUpper(c.Type) {
	case "RSA":
		return c.genRAS()
	case "ED25519":
		return c.genED25519()
	case "ECDSA":
		return c.genECDSA()
	case "X25519":
		return c.genX25519()
	default:
		fmt.Fprintf(os.Stderr, "unsupported key type: %v\n", c.Type)
		return errors.New("unsupported key type")
	}
}
