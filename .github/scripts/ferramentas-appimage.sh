#!/bin/sh
# Baixa o appimagetool e o runtime estático do AppImage em versões fixas, confere o SHA-256 e
# exporta APPIMAGETOOL e APPIMAGE_RUNTIME para os próximos passos (GITHUB_ENV).
# O runtime estático deixa a AppImage abrir sem a libfuse2 (Ubuntu 22.04 em diante não traz).
set -eu
DIR="${RUNNER_TEMP:-/tmp}/ferramentas-appimage"
mkdir -p "$DIR"
baixar() { # url arquivo sha256
  curl -fsSL -H 'User-Agent: cd2-harness' -o "$DIR/$2" "$1"
  echo "$3  $DIR/$2" | sha256sum -c -
}
baixar https://github.com/AppImage/appimagetool/releases/download/1.9.1/appimagetool-x86_64.AppImage \
  appimagetool-x86_64.AppImage ed4ce84f0d9caff66f50bcca6ff6f35aae54ce8135408b3fa33abfc3cb384eb0
baixar https://github.com/AppImage/type2-runtime/releases/download/20251108/runtime-x86_64 \
  runtime-x86_64 2fca8b443c92510f1483a883f60061ad09b46b978b2631c807cd873a47ec260d
chmod +x "$DIR/appimagetool-x86_64.AppImage"
{
  echo "APPIMAGETOOL=$DIR/appimagetool-x86_64.AppImage"
  echo "APPIMAGE_RUNTIME=$DIR/runtime-x86_64"
  # O appimagetool é ele mesmo uma AppImage: no runner, sem FUSE, roda extraído.
  echo "APPIMAGE_EXTRACT_AND_RUN=1"
} >> "${GITHUB_ENV:?}"
