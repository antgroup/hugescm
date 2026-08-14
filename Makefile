SHELL = /usr/bin/env bash -eo pipefail

PKG           := github.com/antgroup/hugescm
SOURCE_DIR    := $(abspath $(dir $(lastword ${MAKEFILE_LIST})))
BUILD_DIR     := ${SOURCE_DIR}/_build
BUILD_TIME    := $(shell date +'%Y-%m-%dT%H:%M:%S%z')
BUILD_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo 'none')
BUILD_VERSION := $(shell cat VERSION || echo '0.30.0')
GO_PACKAGES   := $(shell go list ./... | grep -v '^${PKG}/mock/' | grep -v '^${PKG}/proto/')
GO_LDFLAGS    := -ldflags '-X ${PKG}/pkg/version.version=${BUILD_VERSION} -X ${PKG}/pkg/version.buildTime=${BUILD_TIME} -X ${PKG}/pkg/version.buildCommit=${BUILD_COMMIT}'


.PHONY: all
all: zeta zeta-mc hot

.PHONY: build
build: zeta zeta-mc hot

.PHONY: zeta
zeta:
	GOOS=${BUILD_TARGET} GOARCH=${BUILD_ARCH} go build -C cmd/zeta ${GO_LDFLAGS} -o ${CURDIR}/bin/zeta

.PHONY: zeta-mc
zeta-mc:
	GOOS=${BUILD_TARGET} GOARCH=${BUILD_ARCH} go build -C cmd/zeta-mc ${GO_LDFLAGS} -o ${CURDIR}/bin/zeta-mc

.PHONY: hot
hot:
	GOOS=${BUILD_TARGET} GOARCH=${BUILD_ARCH} go build -C cmd/hot ${GO_LDFLAGS} -o ${CURDIR}/bin/hot

.PHONY: zeta-serve
zeta-serve:
	GOOS=${BUILD_TARGET} GOARCH=${BUILD_ARCH} go build -C cmd/zeta-serve ${GO_LDFLAGS} -o ${CURDIR}/bin/zeta-serve

# Build and package zeta-serve independently from the client.
# Uses cmd/zeta-serve/bali.toml + cmd/zeta-serve/crate.toml so the
# server package does not mix with the client/hot crates that the root
# bali.toml ships.  Outputs to out/zeta-serve/.
# Target-specific defaults so `make zeta-serve-package` works with no
# flags.  BUILD_TARGET/BUILD_ARCH default to the current host's GOOS /
# GOARCH via `go env`; BUILD_VERSION defaults to "1" (the RPM release
# number of the first iteration of ${version from crate.toml}).
# Override with command-line backups:
#   make zeta-serve-package BUILD_TARGET=linux BUILD_ARCH=amd64
#   make zeta-serve-package BUILD_VERSION=2 PACK_FORMAT=sh
PACK_FORMAT ?= tar,rpm
.PHONY: zeta-serve-package
zeta-serve-package: BUILD_TARGET ?= $(shell go env GOOS)
zeta-serve-package: BUILD_ARCH ?= $(shell go env GOARCH)
zeta-serve-package: BUILD_VERSION = 1
zeta-serve-package:
	bali -M cmd/zeta-serve build \
	  -T ${BUILD_TARGET} -A ${BUILD_ARCH} \
	  --release ${BUILD_VERSION} \
	  --pack ${PACK_FORMAT} \
	  -D ${CURDIR}/out/zeta-serve