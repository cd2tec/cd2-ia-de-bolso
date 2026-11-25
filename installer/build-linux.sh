#!/bin/sh
# Compila o instalador para Linux e monta o CD2-IA-de-Bolso-Linux.AppImage em build/bin/.
# Precisa: sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config, e o appimagetool
# (APPIMAGETOOL) com o runtime estático (APPIMAGE_RUNTIME); veja launcher/build/linux/appimage.sh.
# Com CD2_SEM_APPIMAGE=1 só compila o executável (build/bin/cd2-ia-de-bolso), para testar rápido.
# Usa go build direto: a tela já vem pronta em frontend/dist e o wails CLI
# 2.10 quebra ao gerar as ligações com Go novo. As tags são as do wails build.
set -eu
cd "$(dirname "$0")"
VERSAO="${VERSAO:-$(tr -d ' \r\n' < ../VERSION)}"
mkdir -p build/bin
go build -tags desktop,production,webkit2_41 -trimpath \
  -ldflags "-s -w -X main.versao=$VERSAO" \
  -o build/bin/cd2-ia-de-bolso .
[ "${CD2_SEM_APPIMAGE:-0}" = 1 ] && { echo "Pronto: $(pwd)/build/bin/cd2-ia-de-bolso"; exit 0; }
sh ../launcher/build/linux/appimage.sh build/bin/cd2-ia-de-bolso cd2-ia-de-bolso build/linux/cd2-ia-de-bolso.desktop \
  build/appicon.png build/bin/CD2-IA-de-Bolso-Linux.AppImage
