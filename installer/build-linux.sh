#!/bin/sh
# Compila o instalador para Linux (só para desenvolvimento). Saída: build/bin/CD2-IA-de-Bolso
# Precisa: sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config
# Usa go build direto: a tela já vem pronta em frontend/dist e o wails CLI
# 2.10 quebra ao gerar as ligações com Go novo. As tags são as do wails build.
set -e
cd "$(dirname "$0")"
mkdir -p build/bin
go build -tags desktop,production,webkit2_41 -trimpath \
  -ldflags "-s -w -X main.versao=${VERSAO:-dev-linux}" \
  -o build/bin/CD2-IA-de-Bolso .
echo "Pronto: $(pwd)/build/bin/CD2-IA-de-Bolso"
