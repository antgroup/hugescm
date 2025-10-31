// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package serve

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/antgroup/hugescm/modules/base58"
	"golang.org/x/crypto/hkdf"
)

const (
	x25519PublicKeySize = 32
	eciesPrefix         = "ENC@"
)

type Decrypter struct {
	*ecdh.PrivateKey
}

// parsePrivateKey returns pub or pri key through parsing the fname
// Use type assert to get the specific rsa privatekey or publickey
func parsePrivateKey(key []byte) (any, error) {
	block, _ := pem.Decode(key)
	if block == nil {
		return nil, errors.New("malformed Key")
	}
	switch block.Type {
	case "PUBLIC KEY":
		return x509.ParsePKIXPublicKey(block.Bytes)
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		return x509.ParsePKCS8PrivateKey(block.Bytes)
	}
	return nil, fmt.Errorf("key type not supported: %s", block.Type)
}

func NewDecrypter(x25519Key string) (*Decrypter, error) {
	a, err := parsePrivateKey([]byte(x25519Key))
	if err != nil {
		return nil, err
	}
	p, ok := a.(*ecdh.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key type not supported: %s", reflect.TypeOf(a))
	}
	return &Decrypter{PrivateKey: p}, nil
}

func (d *Decrypter) encrypt(plaintext []byte) ([]byte, error) {
	ephemeralPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("gen temp Key error: %w", err)
	}
	ephemeralPublic := ephemeralPriv.PublicKey()
	sharedSecret, err := ephemeralPriv.ECDH(d.PublicKey())
	if err != nil {
		return nil, fmt.Errorf("gen shared Key error: %w", err)
	}
	salt := ephemeralPublic.Bytes()
	info := []byte("ECIES-AES256-GCM-HKDF-SHA256")
	hkdfReader := hkdf.New(sha256.New, sharedSecret, salt, info)
	symmetricKey := make([]byte, 32) // 32 byte for AES-256
	if _, err := io.ReadFull(hkdfReader, symmetricKey); err != nil {
		return nil, fmt.Errorf("gen symmetricKey error: %w", err)
	}
	aesBlock, err := aes.NewCipher(symmetricKey)
	if err != nil {
		return nil, fmt.Errorf("NewCipher %w", err)
	}
	gcm, err := cipher.NewGCM(aesBlock)
	if err != nil {
		return nil, fmt.Errorf("NewGCM %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce error: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return append(ephemeralPublic.Bytes(), ciphertext...), nil
}

func (d *Decrypter) decrypt(message []byte) ([]byte, error) {
	if len(message) < x25519PublicKeySize {
		return nil, errors.New("invalid message size")
	}
	ephemeralPublicBytes := message[:x25519PublicKeySize]
	ciphertext := message[x25519PublicKeySize:]

	ephemeralPublic, err := ecdh.X25519().NewPublicKey(ephemeralPublicBytes)
	if err != nil {
		return nil, fmt.Errorf("cannot gen key: %w", err)
	}

	sharedSecret, err := d.ECDH(ephemeralPublic)
	if err != nil {
		return nil, fmt.Errorf("shared secret bad: %w", err)
	}

	salt := ephemeralPublic.Bytes()
	info := []byte("ECIES-AES256-GCM-HKDF-SHA256")
	hkdfReader := hkdf.New(sha256.New, sharedSecret, salt, info)
	symmetricKey := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, symmetricKey); err != nil {
		return nil, fmt.Errorf("symmetricKey error: %w", err)
	}
	aesBlock, err := aes.NewCipher(symmetricKey)
	if err != nil {
		return nil, fmt.Errorf("NewCipher %w", err)
	}
	gcm, err := cipher.NewGCM(aesBlock)
	if err != nil {
		return nil, fmt.Errorf("NewGCM %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("message too short")
	}
	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid cipher text: %w", err)
	}
	return plaintext, nil
}

func (d *Decrypter) Encrypt(plaintext string) (string, error) {
	b, err := d.encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return eciesPrefix + base58.Encode(b), nil
}

func (d *Decrypter) Decrypt(ciphertext string) (string, error) {
	suffix, ok := strings.CutPrefix(ciphertext, eciesPrefix)
	if !ok {
		return ciphertext, nil
	}
	b, err := d.decrypt(base58.Decode(suffix))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func Encrypt(x25519Key string, plaintext string) (string, error) {
	d, err := NewDecrypter(x25519Key)
	if err != nil {
		return "", err
	}
	return d.Encrypt(plaintext)
}
