#!/bin/sh
# Monta uma AppImage a partir de um executável Linux já compilado (o lançador ou o instalador).
# Uso: sh launcher/build/linux/appimage.sh <executável> <id> <arquivo.desktop> <ícone.png> <saída.AppImage>
# Precisa do appimagetool (APPIMAGETOOL, padrão: appimagetool no PATH) e do runtime estático do
# AppImage (APPIMAGE_RUNTIME, o runtime-x86_64 do AppImage/type2-runtime): com ele a AppImage abre
# sem a libfuse2, que o Ubuntu 22.04 em diante não traz. A WebKitGTK não vai dentro: vem do sistema
# (Ubuntu e Debian com área de trabalho já têm a libwebkit2gtk-4.1).
set -eu
[ "$#" -eq 5 ] || { echo "Uso: $0 <executável> <id> <arquivo.desktop> <ícone.png> <saída.AppImage>"; exit 2; }
EXE="$1"; ID="$2"; DESKTOP="$3"; ICONE="$4"; SAIDA="$5"
TOOL="${APPIMAGETOOL:-appimagetool}"
RUNTIME="${APPIMAGE_RUNTIME:?Defina APPIMAGE_RUNTIME com o runtime estático do AppImage}"
case "$(uname -m)" in aarch64|arm64) ARCH_AI=aarch64 ;; *) ARCH_AI=x86_64 ;; esac
DIR="$(mktemp -d)"
trap 'rm -rf "$DIR"' EXIT
APP="$DIR/AppDir"
mkdir -p "$APP/usr/bin"
cp "$EXE" "$APP/usr/bin/$ID" && chmod 755 "$APP/usr/bin/$ID"
ln -s "usr/bin/$ID" "$APP/AppRun"
cp "$DESKTOP" "$APP/$ID.desktop"
cp "$ICONE" "$APP/$ID.png"
ln -s "$ID.png" "$APP/.DirIcon"
mkdir -p "$(dirname "$SAIDA")"
ARCH="${ARCH:-$ARCH_AI}" "$TOOL" --no-appstream --runtime-file "$RUNTIME" "$APP" "$SAIDA"
chmod 755 "$SAIDA"
echo "Pronto: $SAIDA"
