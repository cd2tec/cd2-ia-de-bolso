#!/usr/bin/env sh
# CD2 Harness - prepara o pen drive (rode UMA vez, com internet, no Mac ou Linux)
# Baixa o llamafile e o modelo para dentro desta pasta. Retoma downloads interrompidos
# e pula o que ja estiver completo, entao pode rodar de novo sem medo.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MODEL_URL="${CD2_MODEL_URL:-https://huggingface.co/unsloth/Qwen3-4B-Instruct-2507-GGUF/resolve/main/Qwen3-4B-Instruct-2507-Q4_K_M.gguf}"
MODEL_FILE="${CD2_MODEL_FILE:-Qwen3-4B-Instruct-2507-Q4_K_M.gguf}"
# SHA-256 do modelo padrao. Com CD2_MODEL_URL proprio, so confere se CD2_MODEL_SHA256 vier junto.
if [ -n "${CD2_MODEL_URL:-}" ]; then MODEL_SHA256="${CD2_MODEL_SHA256:-}"
else MODEL_SHA256="${CD2_MODEL_SHA256:-3605803b982cb64aead44f6c1b2ae36e3acdb41d8e46c8a94c6533bc4c67e597}"; fi
MODEL_SHA256="$(printf '%s' "$MODEL_SHA256" | tr 'A-F' 'a-f')"
# Motor (llamafile): versao e SHA-256 fixos, os mesmos que o manifesto do instalador publica.
# Com CD2_LLAMAFILE_URL proprio, so confere se CD2_LLAMAFILE_SHA256 vier junto.
LLAMAFILE_URL="${CD2_LLAMAFILE_URL:-https://github.com/mozilla-ai/llamafile/releases/download/0.10.6/llamafile-0.10.6}"
if [ -n "${CD2_LLAMAFILE_URL:-}" ]; then LLAMAFILE_SHA256="${CD2_LLAMAFILE_SHA256:-}"
else LLAMAFILE_SHA256="${CD2_LLAMAFILE_SHA256:-d579f61dcd3a306f518e6d90e599d77793ed5f09543023d09c96ad35fcfa63f0}"; fi
LLAMAFILE_SHA256="$(printf '%s' "$LLAMAFILE_SHA256" | tr 'A-F' 'a-f')"
# MCP Toolbox for Databases: versao fixa (pen drive reprodutivel), um binario por sistema.
TOOLBOX_VERSION="${CD2_TOOLBOX_VERSION:-1.14.0}"
TOOLBOX_PLATFORMS="${CD2_TOOLBOX_PLATFORMS:-windows-amd64 darwin-arm64 darwin-amd64 linux-amd64}"
TOOLBOX_BASE="https://storage.googleapis.com/mcp-toolbox-for-databases/v$TOOLBOX_VERSION"
# cd2-ctl (controle do banco pela tela): versao fixa, baixada do release deste repositorio.
# CD2_CTL_LOCAL=1 pula o download e usa o compilado por "sh ctl/build.sh" (antes do 1o release).
CTL_VERSION="${CD2_CTL_VERSION:-0.7.0}"
CTL_BASE="https://github.com/cd2tec/cd2-ia-de-bolso/releases/download/ctl-v$CTL_VERSION"
if command -v shasum >/dev/null 2>&1; then SUM="shasum -a 256"; else SUM="sha256sum"; fi

# Tamanho do arquivo no servidor (segue redirecionamentos). Vazio se nao der para saber.
remote_size() {
  curl -sIL -H 'User-Agent: cd2-harness' "$1" 2>/dev/null | tr -d '\r' \
    | awk 'tolower($1)=="content-length:"{s=$2} END{print s}'
}

# fetch URL DESTINO: pula se ja estiver completo, retoma se estiver pela metade.
fetch() {
  if [ -s "$2" ]; then
    have="$(wc -c < "$2" | tr -d ' ')"
    want="$(remote_size "$1")"
    if [ -n "$want" ] && [ "$want" = "$have" ]; then
      echo "Ja baixado, pulando: $(basename "$2") ($have bytes)"
      return 0
    fi
    # retoma; se o servidor recusar o retomo (ex.: arquivo ja completo mas sem HEAD), baixa inteiro
    curl -L --fail -C - -o "$2" "$1" || curl -L --fail -o "$2" "$1"
  else
    curl -L --fail -C - -o "$2" "$1"
  fi
}

# baixar_motor: o motor nunca e emendado em arquivo de outra versao. O download vai para um pedaco
# com nome tirado da URL (bin/llamafile.part-<12 hex do SHA-256 da URL>), que so a mesma origem
# retoma; pedacos de outras origens sao apagados. So vira bin/llamafile depois de conferir o SHA.
baixar_motor() {
  motor="$ROOT/bin/llamafile"
  if [ -s "$motor" ]; then
    if [ -n "$LLAMAFILE_SHA256" ]; then
      [ "$($SUM "$motor" | cut -d' ' -f1)" = "$LLAMAFILE_SHA256" ] && ok=1 || ok=""
    else
      [ "$(wc -c < "$motor" | tr -d ' ')" = "$(remote_size "$LLAMAFILE_URL")" ] && ok=1 || ok=""
    fi
    if [ -n "$ok" ]; then echo "Ja conferido, pulando: bin/llamafile"
    else echo "O bin/llamafile e de outra versao; baixando o motor fixo."; rm -f "$motor"; fi
  fi
  if [ ! -s "$motor" ]; then
    pedaco="$motor.part-$(printf '%s' "$LLAMAFILE_URL" | $SUM | cut -c1-12)"
    for f in "$motor".part*; do [ -e "$f" ] && [ "$f" != "$pedaco" ] && rm -f "$f"; done
    echo "Origem: $LLAMAFILE_URL"
    # retoma o pedaco; se o servidor recusar o retomo, baixa inteiro
    curl -L --fail -C - -o "$pedaco" "$LLAMAFILE_URL" || { rm -f "$pedaco"; curl -L --fail -o "$pedaco" "$LLAMAFILE_URL"; } \
      || { rm -f "$pedaco"; echo "[ERRO] Falha ao baixar o motor. Rode de novo."; return 1; }
    if [ -n "$LLAMAFILE_SHA256" ]; then
      got="$($SUM "$pedaco" | cut -d' ' -f1)"
      if [ "$got" != "$LLAMAFILE_SHA256" ]; then
        rm -f "$pedaco"; echo "[ERRO] SHA-256 do motor nao confere ($got). Rode de novo."; return 1
      fi
      echo "SHA-256 do motor conferido."
    fi
    mv -f "$pedaco" "$motor"
  fi
  chmod +x "$motor" 2>/dev/null || true
  # Windows exige a extensao .exe; mesma binaria (APE) serve para os tres sistemas
  if ! cmp -s "$motor" "$motor.exe" 2>/dev/null; then
    cp -f "$motor" "$motor.exe"
  fi
}

mkdir -p "$ROOT/bin" "$ROOT/models"
echo "== Motor (llamafile)"
baixar_motor || exit 1

echo "== Modelo"
echo "Origem: $MODEL_URL"
fetch "$MODEL_URL" "$ROOT/models/$MODEL_FILE"

SIZE="$(wc -c < "$ROOT/models/$MODEL_FILE" | tr -d ' ')"
if [ "$SIZE" -lt 1000000000 ]; then echo "[ERRO] Modelo com tamanho suspeito ($SIZE bytes). Apague e rode de novo."; exit 1; fi
if [ -n "$MODEL_SHA256" ]; then
  GOT="$($SUM "$ROOT/models/$MODEL_FILE" | cut -d' ' -f1)"
  if [ "$GOT" != "$MODEL_SHA256" ]; then echo "[ERRO] SHA-256 do modelo nao confere ($GOT). Apague models/$MODEL_FILE e rode de novo."; exit 1; fi
  echo "SHA-256 do modelo conferido."
fi
# O launcher usa o primeiro .gguf que encontrar: fica so o modelo atual em models/.
for f in "$ROOT"/models/*.gguf; do
  [ -e "$f" ] || continue
  [ "$(basename "$f")" = "$MODEL_FILE" ] && continue
  echo "Removendo modelo antigo: $(basename "$f")"; rm -f "$f"
done

echo ""
echo "== Toolbox (MCP Toolbox for Databases v$TOOLBOX_VERSION)"
for plat in $TOOLBOX_PLATFORMS; do
  os="${plat%-*}"; arch="${plat#*-}"; ext=""; [ "$os" = "windows" ] && ext=".exe"
  mkdir -p "$ROOT/bin/toolbox/$plat"
  echo "Origem: $TOOLBOX_BASE/$os/$arch/toolbox$ext"
  fetch "$TOOLBOX_BASE/$os/$arch/toolbox$ext" "$ROOT/bin/toolbox/$plat/toolbox$ext"
  chmod +x "$ROOT/bin/toolbox/$plat/toolbox$ext" 2>/dev/null || true
  TSIZE="$(wc -c < "$ROOT/bin/toolbox/$plat/toolbox$ext" | tr -d ' ')"
  if [ "$TSIZE" -lt 100000000 ]; then echo "[ERRO] Toolbox $plat com tamanho suspeito ($TSIZE bytes). Apague e rode de novo."; exit 1; fi
done

echo ""
echo "== Controle do banco (cd2-ctl v$CTL_VERSION)"
mkdir -p "$ROOT/bin/ctl"
CTL_SUMS=""
if [ "${CD2_CTL_LOCAL:-}" = "1" ]; then
  echo "CD2_CTL_LOCAL=1: usando o cd2-ctl compilado localmente (sh ctl/build.sh)."
else
  CTL_SUMS="$(mktemp "${TMPDIR:-/tmp}/cd2-ctl-sums.XXXXXX")"
  if ! curl -fsSL -H 'User-Agent: cd2-harness' -o "$CTL_SUMS" "$CTL_BASE/SHA256SUMS"; then
    echo "[AVISO] Release ctl-v$CTL_VERSION indisponivel; vou usar o cd2-ctl local, se existir."
    rm -f "$CTL_SUMS"; CTL_SUMS=""
  fi
fi
for plat in $TOOLBOX_PLATFORMS; do
  os="${plat%-*}"; ext=""; [ "$os" = "windows" ] && ext=".exe"
  asset="cd2-ctl-$plat$ext"; dest="$ROOT/bin/ctl/$plat/cd2-ctl$ext"
  mkdir -p "$ROOT/bin/ctl/$plat"
  if [ -n "$CTL_SUMS" ]; then
    want="$(awk -v f="$asset" '$2 == f || $2 == "*" f { print $1 }' "$CTL_SUMS")"
    got=""
    [ -s "$dest" ] && got="$($SUM "$dest" | cut -d' ' -f1)"
    if [ -n "$want" ] && [ "$want" = "$got" ]; then
      echo "Ja conferido, pulando: bin/ctl/$plat/cd2-ctl$ext"
    else
      # sem retomar (-C -): o arquivo que esta ai e de outra versao, emendar nele corromperia o binario
      echo "Origem: $CTL_BASE/$asset"
      rm -f "$dest"
      if ! curl -L --fail -H 'User-Agent: cd2-harness' -o "$dest" "$CTL_BASE/$asset"; then
        rm -f "$dest" "$CTL_SUMS"
        echo "[ERRO] Falha ao baixar o cd2-ctl $plat. Rode de novo."; exit 1
      fi
      got="$($SUM "$dest" | cut -d' ' -f1)"
    fi
    if [ -z "$want" ] || [ "$want" != "$got" ]; then
      rm -f "$dest" "$CTL_SUMS"
      echo "[ERRO] cd2-ctl $plat nao confere com o SHA256SUMS do release. Rode de novo."; exit 1
    fi
  elif [ -s "$dest" ]; then
    echo "Usando o cd2-ctl local: bin/ctl/$plat/cd2-ctl$ext"
  else
    echo "[ERRO] Sem cd2-ctl para $plat. Publique a tag ctl-v$CTL_VERSION ou compile com: sh ctl/build.sh \"$plat\" e rode com CD2_CTL_LOCAL=1."
    exit 1
  fi
  chmod +x "$dest" 2>/dev/null || true
done
if [ -n "$CTL_SUMS" ]; then rm -f "$CTL_SUMS"; fi

echo ""
echo "== Checksums (anote para conferir depois)"
# Os lancadores da raiz entram so se estiverem na copia. Os nomes tem espaco, entao vao por "$@".
LAUNCHERS=""
[ -f "$ROOT/IA de Bolso.app/Contents/MacOS/IA de Bolso" ] && LAUNCHERS="app"
[ -f "$ROOT/IA de Bolso.exe" ] && LAUNCHERS="$LAUNCHERS exe"
set --
case "$LAUNCHERS" in *app*) set -- "$@" "IA de Bolso.app/Contents/MacOS/IA de Bolso" ;; esac
case "$LAUNCHERS" in *exe*) set -- "$@" "IA de Bolso.exe" ;; esac
( cd "$ROOT" && $SUM "$@" bin/llamafile "models/$MODEL_FILE" $(ls bin/toolbox/*/toolbox bin/toolbox/*/toolbox.exe bin/ctl/*/cd2-ctl bin/ctl/*/cd2-ctl.exe 2>/dev/null) | tee CHECKSUMS.sha256 )
echo ""
echo "Pronto. Pen drive preparado em: $ROOT"
echo "Dica: formate o pen drive em exFAT (FAT32 nao aceita arquivos acima de 4 GB)."
