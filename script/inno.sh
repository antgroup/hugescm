#!/usr/bin/env bash

REALPATH=$(realpath "$0")
SCRIPTROOT=$(dirname "$REALPATH")
TOPLEVEL=$(dirname "$SCRIPTROOT")

# &$InnoSetup "$BaulkIss" "/dArchitecturesAllowed=$ArchitecturesAllowed" "/dArchitecturesInstallIn64BitMode=$ArchitecturesInstallIn64BitMode" "/dInstallTarget=user"

echo -e "build root \\x1b[32m${TOPLEVEL}\\x1b[0m"
OLDPWD=$(pwd)
cd "$TOPLEVEL" || exit 1
VERSION=$(cat VERSION) || exit 1
bali --target=windows --arch=amd64 || exit 1

docker run --rm -i -v "$TOPLEVEL:/work" amake/innosetup "/dAppVersion=${VERSION}" "/dArchitecturesAllowed=x64compatible" "/dArchitecturesInstallIn64BitMode=x64compatible" "/dInstallTarget=user" script/zeta.iss
docker run --rm -i -v "$TOPLEVEL:/work" amake/innosetup "/dAppVersion=${VERSION}" "/dArchitecturesAllowed=x64compatible" "/dArchitecturesInstallIn64BitMode=x64compatible" "/dInstallTarget=admin" script/zeta.iss

bali --target=windows --arch=arm64 || exit 1

docker run --rm -i -v "$TOPLEVEL:/work" amake/innosetup "/dAppVersion=${VERSION}" "/dArchitecturesAllowed=arm64" "/dArchitecturesInstallIn64BitMode=arm64" "/dInstallTarget=user" script/zeta.iss
docker run --rm -i -v "$TOPLEVEL:/work" amake/innosetup "/dAppVersion=${VERSION}" "/dArchitecturesAllowed=arm64" "/dArchitecturesInstallIn64BitMode=arm64" "/dInstallTarget=admin" script/zeta.iss
