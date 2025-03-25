// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package serve

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"regexp"
)

type Decryptor struct {
	*rsa.PrivateKey
}

// parseRsaKey returns pub or pri key through parsing the fname
// Use type assert to get the specific rsa privatekey or publickey
func parseRsaKey(key []byte) (any, error) {
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

func NewDecryptor(decryptedKey string) (*Decryptor, error) {
	rsaKey, err := parseRsaKey([]byte(decryptedKey))
	if err != nil {
		return nil, err
	}
	if k, ok := rsaKey.(*rsa.PrivateKey); ok {
		return &Decryptor{PrivateKey: k}, nil
	}
	return nil, errors.New("not rsa key")
}

func (d *Decryptor) Decrypt(data []byte) ([]byte, error) {
	chunkLen := d.N.BitLen() / 8
	var b bytes.Buffer
	chunkNum := int(math.Ceil(float64(len(data)) / float64(chunkLen)))
	var err error
	var decryptedPart []byte
	for i := range chunkNum {
		if i == chunkNum-1 {
			decryptedPart, err = rsa.DecryptPKCS1v15(rand.Reader, d.PrivateKey, data[chunkLen*i:])
		} else {
			decryptedPart, err = rsa.DecryptPKCS1v15(rand.Reader, d.PrivateKey, data[chunkLen*i:chunkLen*(i+1)])
		}
		if err != nil {
			return nil, err
		}
		b.Write(decryptedPart)
	}
	return b.Bytes(), nil
}

func (d *Decryptor) Encrypt(data []byte) ([]byte, error) {
	chunkLen := d.N.BitLen()/8 - 11
	var b bytes.Buffer
	chunkNum := int(math.Ceil(float64(len(data)) / float64(chunkLen)))
	var err error
	var encryptedPart []byte
	for i := range chunkNum {
		if i == chunkNum-1 {
			encryptedPart, err = rsa.EncryptPKCS1v15(rand.Reader, &d.PublicKey, data[chunkLen*i:])
		} else {
			encryptedPart, err = rsa.EncryptPKCS1v15(rand.Reader, &d.PublicKey, data[chunkLen*i:chunkLen*(i+1)])
		}
		if err != nil {
			return nil, err
		}
		b.Write(encryptedPart)
	}
	return b.Bytes(), nil
}

var (
	regEncryptBlock = regexp.MustCompile(`^ENC\((?:[A-Za-z0-9+\\/]{4})*(?:[A-Za-z0-9+\\/]{2}==|[A-Za-z0-9+\\/]{3}=|[A-Za-z0-9+\\/]{4})\)$`)
)

func Decrypt(content string, decryptedKey string) (string, error) {
	if !regEncryptBlock.MatchString(content) {
		return content, nil
	}
	encryptedRaw, err := base64.StdEncoding.DecodeString(content[4 : len(content)-1])
	if err != nil {
		return "", err
	}
	d, err := NewDecryptor(decryptedKey)
	if err != nil {
		return "", err
	}
	rawContent, err := d.Decrypt(encryptedRaw)
	if err != nil {
		return "", err
	}
	return string(rawContent), nil
}

func Encrypt(content string, decryptedKey string) (string, error) {
	d, err := NewDecryptor(decryptedKey)
	if err != nil {
		return "", err
	}
	encryptedData, err := d.Encrypt([]byte(content))
	if err != nil {
		return "", err
	}
	hexData := base64.StdEncoding.EncodeToString(encryptedData)
	return "ENC(" + hexData + ")", nil
}
