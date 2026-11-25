#!/bin/sh
# Compila o lançador para Linux e monta o "IA de Bolso.AppImage" em build/bin/.
# Precisa: sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config, e o appimagetool
# (APPIMAGETOOL) com o runtime estático (APPIMAGE_RUNTIME); veja build/linux/appimage.sh.
# Usa go build direto, como o installer/build-linux.sh: a tela já vem pronta em frontend/dist e o
# wails CLI 2.10 quebra ao gerar as ligações com Go novo. As tags são as do wails build; a
# webkit2_41 usa a WebKitGTK 4.1, a única do Ubuntu 24.04.
set -eu
cd "$(dirname "$0")"
VERSAO="${VERSAO:-$(tr -d ' \r\n' < ../VERSION)}"
mkdir -p build/bin
go build -tags desktop,production,webkit2_41 -trimpath \
  -ldflags "-s -w -X main.versao=$VERSAO" \
  -o build/bin/ia-de-bolso .
sh build/linux/appimage.sh build/bin/ia-de-bolso ia-de-bolso build/linux/ia-de-bolso.desktop \
  build/appicon.png "build/bin/IA de Bolso.AppImage"
