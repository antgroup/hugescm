// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"embed"
	"io/fs"
)

//go:embed web/templates/*.tmpl web/templates/partials/*.tmpl
var templateFS embed.FS

//go:embed web/static
var staticFS embed.FS

// templateFileSystem returns the embedded templates as an fs.FS.
func templateFileSystem() fs.FS {
	f, err := fs.Sub(templateFS, "web/templates")
	if err != nil {
		panic(err)
	}
	return f
}

// staticFileSystem returns the embedded static files as an fs.FS.
func staticFileSystem() fs.FS {
	f, err := fs.Sub(staticFS, "web/static")
	if err != nil {
		panic(err)
	}
	return f
}
