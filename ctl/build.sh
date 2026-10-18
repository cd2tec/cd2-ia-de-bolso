#!/usr/bin/env sh
# Compila o cd2-ctl para bin/ctl/<os>-<arch>/ (desenvolvimento e teste local; o release
# oficial sai do GitHub Actions, workflow ctl-release.yml).
# Uso: sh ctl/build.sh                         (plataforma deste computador)
#      CD2_GO_TAGS=sandbox sh ctl/build.sh     (build de teste: Worker e chave da sandbox)
#      sh ctl/build.sh "darwin-arm64 linux-amd64"
set -eu
HERE="$(cd "$(dirname "$0")" && pwd)"; ROOT="$(cd "$HERE/.." && pwd)"
PLATS="${1:-$(go env GOOS)-$(go env GOARCH)}"
VERSION="$(tr -d ' \n' < "$ROOT/VERSION")-local"
cd "$HERE"
for p in $PLATS; do
  os="${p%-*}"; arch="${p#*-}"; ext=""; [ "$os" = windows ] && ext=".exe"
  mkdir -p "$ROOT/bin/ctl/$p"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -tags "${CD2_GO_TAGS:-}" -ldflags "-s -w -X main.version=$VERSION" -o "$ROOT/bin/ctl/$p/cd2-ctl$ext" .
  echo "bin/ctl/$p/cd2-ctl$ext"
done
