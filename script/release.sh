#!/usr/bin/env bash

SCRIPT_FOLDER_REL=$(dirname "$0")
SCRIPT_FOLDER=$(
	cd "${SCRIPT_FOLDER_REL}" || exit
	pwd
)
TOPLEVEL_SOURCE_DIR=$(dirname "${SCRIPT_FOLDER}")

go install github.com/balibuild/bali/v3/cmd/bali@latest

cd "${TOPLEVEL_SOURCE_DIR}" || exit 1

case "$OSTYPE" in
solaris*)
	echo "solaris unsupported"
	;;
darwin*)
	echo -e "build for \x1b[32mdarwin/amd64\x1b[0m"
	if ! bali '--pack=tar,sh' --target=darwin --arch=amd64; then
		echo "build HugeSCM failed"
		exit 1
	fi
	echo -e "build for \x1b[32mdarwin/arm64\x1b[0m"
	if ! bali '--pack=tar,sh' --target=darwin --arch=arm64; then
		echo "build HugeSCM failed"
		exit 1
	fi
	;;
linux*)
	echo -e "build for \x1b[32mlinux/amd64\x1b[0m"
	if ! bali --pack='rpm,deb,tar,sh' --target=linux --arch=amd64; then
		echo "build HugeSCM failed"
		exit 1
	fi
	echo -e "build for \x1b[32mlinux/arm64\x1b[0m"
	if ! bali --pack='rpm,deb,tar,sh' --target=linux --arch=arm64; then
		echo "build HugeSCM failed"
		exit 1
	fi
	echo -e "build for \x1b[32mdarwin/amd64\x1b[0m"
	if ! bali '--pack=tar,sh' --target=darwin --arch=amd64; then
		echo "build HugeSCM failed"
		exit 1
	fi
	echo -e "build for \x1b[32mdarwin/arm64\x1b[0m"
	if ! bali '--pack=tar,sh' --target=darwin --arch=arm64; then
		echo "build HugeSCM failed"
		exit 1
	fi
	;;
bsd*)
	echo "bsd unsupported"
	;;
msys*)
	echo -e "build for \x1b[32mwindows/amd64\x1b[0m"
	if ! bali --pack=zip --target=windows --arch=amd64; then
		echo "build HugeSCM failed"
		exit 1
	fi
	echo -e "build for \x1b[32mwindows/arm64\x1b[0m"
	if ! bali --pack=zip --target=windows --arch=arm64; then
		echo "build HugeSCM failed"
		exit 1
	fi
	;;
esac

echo -e "\\x1b[32mHugeSCM: build success\\x1b[0m"
