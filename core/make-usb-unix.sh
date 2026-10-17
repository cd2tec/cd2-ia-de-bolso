#!/usr/bin/env sh
# CD2 Harness - grava um pen drive pronto para uso (macOS e Linux, POSIX sh)
# Uso: sh core/make-usb-unix.sh        (ou duplo clique em GRAVAR-PENDRIVE-Mac.command)
#
# O que faz, nesta ordem:
#   1. monta uma copia local do harness e baixa motor + modelo (ficam em cache; o
#      segundo pen drive nao baixa nada de novo)
#   2. APAGA o pen drive escolhido e formata em exFAT (MBR), nome CD2
#   3. copia os arquivos para a raiz do pen drive (ele acabou de ser formatado,
#      entao nao precisa de subpasta)
#   4. confere os checksums direto no pen drive e ejeta
#
# Variaveis opcionais: CD2_USB_DISK (ex.: /dev/disk5 ou /dev/sdb) pula a pergunta de
# qual disco; CD2_USB_YES=1 pula a confirmacao "APAGAR" (para gravar em serie).
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OS="$(uname -s)"
LABEL="${CD2_USB_LABEL:-CD2}"
MIN_BYTES=4000000000

say() { printf '%s\n' "$*"; }
die() { printf '\n[ERRO] %s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "Preciso do comando '$1'. $2"; }

case "$OS" in
  Darwin) CACHE="$HOME/Library/Caches/cd2-harness/usb" ;;
  Linux)  CACHE="${XDG_CACHE_HOME:-$HOME/.cache}/cd2-harness/usb" ;;
  *) die "Sistema nao suportado: $OS. No Windows use GRAVAR-PENDRIVE-Windows.bat." ;;
esac
STAGE="$CACHE/CD2-HARNESS"   # copia local no cache (o nome antigo fica para nao baixar tudo de novo); no pen drive vai para a raiz
if command -v shasum >/dev/null 2>&1; then SUM="shasum -a 256"; else SUM="sha256sum"; fi

printf '\n'
printf '  \033[38;5;179m+-----------------------------+\033[0m\n'
printf '  \033[38;5;179m|\033[0m   \033[1mCD2 Tech\033[0m  IA de Bolso     \033[38;5;179m|\033[0m\n'
printf '  \033[38;5;179m+-----------------------------+\033[0m\n'
printf '  Gravar pen drive\n\n'

# --- 0. escolher o pen drive --------------------------------------------------
list_disks() {
  if [ "$OS" = "Darwin" ]; then
    diskutil list external physical 2>/dev/null | grep -Eo '^/dev/disk[0-9]+' | while read -r d; do
      info="$(diskutil info "$d" 2>/dev/null || true)"
      size="$(printf '%s\n' "$info" | sed -n 's/^ *Disk Size: *\([^(]*\) (.*/\1/p' | sed 's/ *$//')"
      name="$(printf '%s\n' "$info" | sed -n 's/^ *Device \/ Media Name: *//p')"
      printf '%s\t%s\t%s\n' "$d" "$size" "$name"
    done
  else
    need lsblk "Instale o pacote util-linux."
    lsblk -dnpo NAME,SIZE,TRAN,RM,TYPE,MODEL 2>/dev/null | awk '{
      n=$1; s=$2; t=$3; r=$4; ty=$5; $1=$2=$3=$4=$5=""; sub(/^ +/, "");
      if (ty=="disk" && (t=="usb" || r=="1")) printf "%s\t%s\t%s\n", n, s, $0 }'
  fi
}
disk_bytes() {
  if [ "$OS" = "Darwin" ]; then diskutil info "$1" | sed -n 's/^ *Disk Size:.*(\([0-9]*\) Bytes).*/\1/p'
  else lsblk -dnbo SIZE "$1"; fi
}

CANDS="$(list_disks)"
[ -n "$CANDS" ] || die "Nenhum pen drive encontrado. Plugue o pen drive e rode de novo."
say "Pen drives encontrados:"
printf '%s\n' "$CANDS" | awk -F'\t' '{ printf "  %-14s %-10s %s\n", $1, $2, $3 }'
say ""
DEFAULT=""
[ "$(printf '%s\n' "$CANDS" | wc -l | tr -d ' ')" -eq 1 ] && DEFAULT="$(printf '%s\n' "$CANDS" | cut -f1)"
if [ -n "${CD2_USB_DISK:-}" ]; then
  DISK="$CD2_USB_DISK"
else
  printf 'Qual deles? Digite o identificador%s: ' "${DEFAULT:+ [$DEFAULT]}"
  read -r DISK || DISK=""
  [ -n "$DISK" ] || DISK="$DEFAULT"
fi
[ -n "$DISK" ] || { say "Cancelado. Nada foi apagado."; exit 0; }
printf '%s\n' "$CANDS" | cut -f1 | grep -qx "$DISK" || die "'$DISK' nao esta na lista de pen drives acima. Nada foi apagado."
BYTES="$(disk_bytes "$DISK" | tr -d ' ')"
[ -n "$BYTES" ] && [ "$BYTES" -lt "$MIN_BYTES" ] && die "Esse pen drive tem menos de 4 GB; a IA de Bolso precisa de uns 3 GB livres."

if [ "${CD2_USB_YES:-}" != "1" ]; then
  say ""
  say "ATENCAO: TUDO que esta em $DISK sera apagado."
  printf 'Digite APAGAR para continuar (qualquer outra coisa cancela): '
  read -r OK || OK=""
  [ "$OK" = "APAGAR" ] || { say "Cancelado. Nada foi apagado."; exit 0; }
fi

# --- 1. copia local + downloads (cache) ---------------------------------------
say ""
say "== 1/4 Montando a copia local em $STAGE"
mkdir -p "$STAGE"
for f in "$STAGE"/* "$STAGE"/.[!.]*; do
  [ -e "$f" ] || continue
  case "$(basename "$f")" in bin|models|CHECKSUMS.sha256) ;; *) rm -rf "$f" ;; esac
done
for item in INICIAR-CD2-Windows.bat INICIAR-CD2-Mac.command iniciar-cd2-linux.sh README.md LICENSE VERSION web core; do
  [ -e "$ROOT/$item" ] || die "Nao achei '$item' em $ROOT. Rode este script de dentro da pasta do repositorio."
  if [ "$OS" = "Darwin" ]; then cp -RX "$ROOT/$item" "$STAGE/"; else cp -R "$ROOT/$item" "$STAGE/"; fi
done
# Lancadores da raiz ("IA de Bolso.exe" e "IA de Bolso.app"): entram so se existirem na copia.
# No Mac e sem atributos estendidos (-X): a quarentena iria para o ._ do exFAT e o Gatekeeper bloquearia o .app.
for item in "IA de Bolso.exe" "IA de Bolso.app"; do
  [ -e "$ROOT/$item" ] || continue
  if [ "$OS" = "Darwin" ]; then cp -RX "$ROOT/$item" "$STAGE/"; else cp -R "$ROOT/$item" "$STAGE/"; fi
done
rm -f "$STAGE/core/make-usb-unix.sh" "$STAGE/core/make-usb-windows.ps1"
# Com o lancador do Mac na copia, o .command vai para core/; sem ele, fica na raiz (plano B). O .bat fica na raiz.
. "$ROOT/core/lancadores.sh"
arrumar_lancadores "$STAGE" || die "Nao consegui arrumar os lancadores na copia local."
find "$STAGE" \( -name '.DS_Store' -o -name '._*' \) -delete 2>/dev/null || true

say ""
say "Baixando motor e modelo (uns 3,9 GB na primeira vez: motor, modelo, toolbox e cd2-ctl; depois fica em cache)..."
sh "$STAGE/core/prepare-unix.sh" || die "O preparo (download) falhou. Rode de novo: ele retoma de onde parou."

# --- 2. formatar ----------------------------------------------------------------
say ""
say "== 2/4 Formatando $DISK em exFAT ($LABEL)"
if [ "$OS" = "Darwin" ]; then
  diskutil unmountDisk force "$DISK" >/dev/null 2>&1 || true
  diskutil eraseDisk ExFAT "$LABEL" MBR "$DISK" || die "A formatacao falhou."
  PART="${DISK}s1"
  MNT="$(diskutil info "$PART" | sed -n 's/^ *Mount Point: *//p')"
  if [ ! -d "$MNT" ]; then diskutil mount "$PART" >/dev/null; MNT="$(diskutil info "$PART" | sed -n 's/^ *Mount Point: *//p')"; fi
  [ -d "$MNT" ] || die "Formatei, mas nao achei o ponto de montagem de $PART."
else
  need parted "Instale: sudo apt install parted (ou dnf install parted)."
  need mkfs.exfat "Instale: sudo apt install exfatprogs (ou exfat-utils)."
  SUDO=""; [ "$(id -u)" -eq 0 ] || { need sudo "Rode como root."; SUDO="sudo"; }
  for p in $(lsblk -lnpo NAME "$DISK" | tail -n +2); do $SUDO umount "$p" >/dev/null 2>&1 || true; done
  $SUDO wipefs -a "$DISK" >/dev/null
  $SUDO parted -s "$DISK" mklabel msdos mkpart primary 1MiB 100% || die "A formatacao (parted) falhou."
  $SUDO partprobe "$DISK" >/dev/null 2>&1 || true
  command -v udevadm >/dev/null 2>&1 && udevadm settle >/dev/null 2>&1 || sleep 2
  PART="$(lsblk -lnpo NAME "$DISK" | sed -n 2p)"
  [ -n "$PART" ] || die "Nao achei a particao criada em $DISK."
  # exfatprogs usa -L, exfat-utils usa -n
  $SUDO mkfs.exfat -L "$LABEL" "$PART" >/dev/null 2>&1 || $SUDO mkfs.exfat -n "$LABEL" "$PART" >/dev/null || die "mkfs.exfat falhou."
  command -v udevadm >/dev/null 2>&1 && udevadm settle >/dev/null 2>&1 || sleep 2
  MNT="$(findmnt -no TARGET "$PART" 2>/dev/null || true)"
  if [ -z "$MNT" ]; then
    MNT="$(mktemp -d /tmp/cd2-usb.XXXXXX)"
    $SUDO mount -o "uid=$(id -u),gid=$(id -g)" "$PART" "$MNT" || die "Nao consegui montar $PART."
    UNMOUNT_ME=1
  fi
fi

# --- 3. copiar ------------------------------------------------------------------
say ""
say "== 3/4 Copiando para a raiz de $MNT (uns 3 GB; pode levar alguns minutos)"
if [ "$OS" = "Darwin" ]; then cp -RX "$STAGE"/. "$MNT/"; else cp -R "$STAGE"/. "$MNT/"; fi
find "$MNT" \( -name '._*' -o -name '.DS_Store' \) -delete 2>/dev/null || true
sync

# --- 4. conferir e ejetar -------------------------------------------------------
say ""
say "== 4/4 Conferindo os checksums direto no pen drive"
( cd "$MNT" && $SUM -c CHECKSUMS.sha256 ) || die "Checksum diferente no pen drive. Nao use este pen drive; grave de novo."
mkdir -p "$CACHE"
KEEP="$CACHE/CHECKSUMS-$(date +%Y%m%d-%H%M%S).sha256"
cp "$MNT/CHECKSUMS.sha256" "$KEEP"
say ""
say "Conteudo gravado:"
( cd "$MNT" && ls -lh bin bin/toolbox/* bin/ctl/* models CHECKSUMS.sha256 | sed 's/^/  /' )
sync
if [ "$OS" = "Darwin" ]; then
  diskutil eject "$DISK" >/dev/null && say "Ejetado." || say "Nao consegui ejetar sozinha; ejete pelo Finder antes de tirar."
else
  if [ "${UNMOUNT_ME:-}" = "1" ]; then $SUDO umount "$MNT" && rmdir "$MNT"; else $SUDO umount "$MNT" 2>/dev/null || true; fi
  $SUDO eject "$DISK" >/dev/null 2>&1 || udisksctl power-off -b "$DISK" >/dev/null 2>&1 || true
  say "Desmontado."
fi

say ""
say "Pronto. Pode tirar o pen drive."
say "Copia do checksum guardada em: $KEEP"
say "Para gravar outro pen drive, plugue o proximo e rode de novo: nao baixa nada de novo."
