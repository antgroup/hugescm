// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

// ValidateDocument validates that a Document can be successfully
// decoded into a valid Config struct.
func ValidateDocument(doc Document) error {
	var cfg Config
	return ValidateDocumentAs(doc, &cfg)
}
