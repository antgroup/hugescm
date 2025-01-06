SHELL = /usr/bin/env bash -eo pipefail

PKG           := github.com/antgroup/hugescm
SOURCE_DIR    := $(abspath $(dir $(lastword ${MAKEFILE_LIST})))
BUILD_DIR     := ${SOURCE_DIR}/_build
BUILD_TIME    := $(shell date +'%Y-%m-%dT%H:%M:%S%z')
BUILD_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo 'none')
BUILD_VERSION := $(shell cat VERSION || echo '0.16.2')
GO_PACKAGES   := $(shell go list ./... | grep -v '^${PKG}/mock/' | grep -v '^${PKG}/proto/')
GO_LDFLAGS    := -ldflags '-X ${PKG}/pkg/version.version=${BUILD_VERSION} -X ${PKG}/pkg/version.buildTime=${BUILD_TIME} -X ${PKG}/pkg/version.buildCommit=${BUILD_COMMIT}'


.PHONY: all
all: zeta zeta-mc

.PHONY: build
build: zeta zeta-mc

.PHONY: zeta
zeta:
	GOOS=${BUILD_TARGET} GOARCH=${BUILD_ARCH} go build -C cmd/zeta ${GO_LDFLAGS} -o ${CURDIR}/bin/zeta

.PHONY: zeta-mc
zeta-mc:
	GOOS=${BUILD_TARGET} GOARCH=${BUILD_ARCH} go build -C cmd/zeta-mc ${GO_LDFLAGS} -o ${CURDIR}/bin/zeta-mc

.PHONY: zeta-serve
zeta-serve:
	GOOS=${BUILD_TARGET} GOARCH=${BUILD_ARCH} go build -C cmd/zeta-serve ${GO_LDFLAGS} -o ${CURDIR}/bin/zeta-serve