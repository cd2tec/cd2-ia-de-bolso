#!/bin/bash
# Assina e notariza o que vai para o Mac: o .app (lançador e instalador) e o .dmg do instalador.
#
#   assinar-mac.sh app "<caminho>.app"   assina; com notarização, envia, grampeia e confere no Gatekeeper
#   assinar-mac.sh dmg "<caminho>.dmg"   o mesmo para o dmg (o .app de dentro já deve ter passado pelo "app")
#
# De onde vem a assinatura, na ordem:
#   1. CI: MAC_CERT_P12_BASE64 + MAC_CERT_PASSWORD (o .p12 do Developer ID, importado num keychain temporário).
#   2. Local: MAC_IDENTIDADE com o nome do certificado já instalado no keychain.
#   3. Nada disso: assinatura ad hoc, que o Apple Silicon exige para abrir. O Gatekeeper avisa quando o .app
#      chega com quarentena (baixado pelo navegador); copiado sem quarentena para o pen drive, abre direto.
# De onde vem a notarização:
#   1. CI: ASC_KEY_P8_BASE64 + ASC_KEY_ID + ASC_ISSUER_ID (chave da App Store Connect API).
#   2. Local: MAC_NOTARY_PROFILE com o perfil salvo por "xcrun notarytool store-credentials".
#   Sem nenhum dos dois, assina com Developer ID e não notariza (o Gatekeeper ainda avisa).
# Secret pela metade é erro, não fallback: senão um secret com nome errado vira release ad hoc sem ninguém notar.
set -euo pipefail

MODO="${1:-}"
ALVO="${2:-}"
[ "$MODO" = app ] || [ "$MODO" = dmg ] || { echo "Uso: $0 app|dmg <caminho>"; exit 2; }
[ -e "$ALVO" ] || { echo "Não achei: $ALVO"; exit 2; }

aviso() {
  if [ -n "${GITHUB_ACTIONS:-}" ]; then echo "::warning::$1"; else echo "AVISO: $1"; fi
}

meio_preenchido() {
  local cheios=0 total=0 v
  for v in "$@"; do total=$((total + 1)); [ -n "${!v:-}" ] && cheios=$((cheios + 1)); done
  [ "$cheios" -gt 0 ] && [ "$cheios" -lt "$total" ]
}
if meio_preenchido MAC_CERT_P12_BASE64 MAC_CERT_PASSWORD; then
  echo "Faltou um dos secrets do certificado: MAC_CERT_P12_BASE64 e MAC_CERT_PASSWORD vão juntos."; exit 1
fi
if meio_preenchido ASC_KEY_P8_BASE64 ASC_KEY_ID ASC_ISSUER_ID; then
  echo "Faltou um dos secrets da notarização: ASC_KEY_P8_BASE64, ASC_KEY_ID e ASC_ISSUER_ID vão juntos."; exit 1
fi

TEMP="${RUNNER_TEMP:-${TMPDIR:-/tmp}}"

# --- Identidade de assinatura ---
IDENTIDADE=""
if [ -n "${MAC_CERT_P12_BASE64:-}" ]; then
  # Keychain próprio, reaproveitado se o job chamar o script duas vezes (app e depois dmg).
  KC="$TEMP/cd2-assinatura.keychain-db"
  KC_SENHA="cd2-assinatura-ci"
  if [ ! -f "$KC" ]; then
    P12="$TEMP/cd2-developer-id.p12"
    printf '%s' "$MAC_CERT_P12_BASE64" | base64 --decode > "$P12"
    security create-keychain -p "$KC_SENHA" "$KC"
    security set-keychain-settings -lut 21600 "$KC"
    security unlock-keychain -p "$KC_SENHA" "$KC"
    security import "$P12" -k "$KC" -P "$MAC_CERT_PASSWORD" -T /usr/bin/codesign -T /usr/bin/security
    rm -f "$P12"
    security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$KC_SENHA" "$KC" >/dev/null
    # Entra na lista de busca, senão o codesign não acha a chave.
    security list-keychains -d user -s "$KC" $(security list-keychains -d user | tr -d '"')
  fi
  IDENTIDADE="$(security find-identity -v -p codesigning "$KC" | grep -o '"Developer ID Application: [^"]*"' | head -1 | tr -d '"')"
  [ -n "$IDENTIDADE" ] || { echo "O .p12 não trouxe um certificado Developer ID Application válido."; exit 1; }
elif [ -n "${MAC_IDENTIDADE:-}" ]; then
  IDENTIDADE="$MAC_IDENTIDADE"
fi

# --- Credencial da notarização ---
NOTARY=()
if [ -n "${ASC_KEY_P8_BASE64:-}" ]; then
  P8="$TEMP/cd2-asc-key.p8"
  printf '%s' "$ASC_KEY_P8_BASE64" | base64 --decode > "$P8"
  NOTARY=(--key "$P8" --key-id "$ASC_KEY_ID" --issuer "$ASC_ISSUER_ID")
elif [ -n "${MAC_NOTARY_PROFILE:-}" ]; then
  NOTARY=(--keychain-profile "$MAC_NOTARY_PROFILE")
fi

notarizar() {
  # $1: o que vai para a Apple (zip ou dmg). A saída em JSON evita depender do código de saída do notarytool.
  local envio="$1" saida id status
  saida="$(xcrun notarytool submit "$envio" "${NOTARY[@]}" --wait --output-format json)"
  id="$(printf '%s' "$saida" | plutil -extract id raw -o - -)"
  status="$(printf '%s' "$saida" | plutil -extract status raw -o - -)"
  echo "Notarização $id: $status"
  if [ "$status" != Accepted ]; then
    xcrun notarytool log "$id" "${NOTARY[@]}" || true
    exit 1
  fi
}

# --- Sem Developer ID: ad hoc ---
if [ -z "$IDENTIDADE" ]; then
  if [ "$MODO" = app ]; then
    aviso "Sem certificado Developer ID: $(basename "$ALVO") assinado ad hoc, sem notarização."
    codesign --force --deep -s - "$ALVO"
    codesign --verify --deep --strict "$ALVO"
  else
    aviso "Sem certificado Developer ID: $(basename "$ALVO") vai sem assinatura."
  fi
  exit 0
fi

# --- Developer ID ---
# Hardened Runtime (--options runtime) e --timestamp são exigências da notarização.
echo "Assinando $(basename "$ALVO") com: $IDENTIDADE"
if [ "$MODO" = app ]; then
  codesign --force --deep --options runtime --timestamp -s "$IDENTIDADE" "$ALVO"
  codesign --verify --deep --strict "$ALVO"
else
  codesign --force --timestamp -s "$IDENTIDADE" "$ALVO"
  codesign --verify "$ALVO"
fi

if [ ${#NOTARY[@]} -eq 0 ]; then
  aviso "Sem credencial de notarização: $(basename "$ALVO") assinado com Developer ID, mas não notarizado."
  exit 0
fi

if [ "$MODO" = app ]; then
  # O notarytool não aceita .app solto: vai zipado, e o ticket é grampeado no próprio .app.
  ENVIO="$TEMP/cd2-notarizar-$$.zip"
  ditto -c -k --keepParent "$ALVO" "$ENVIO"
  notarizar "$ENVIO"
  rm -f "$ENVIO"
else
  notarizar "$ALVO"
fi

# O ticket grampeado deixa o Gatekeeper validar sem internet, que é o caso do pen drive.
xcrun stapler staple "$ALVO"
xcrun stapler validate "$ALVO"
if [ "$MODO" = app ]; then
  spctl --assess --type execute -vv "$ALVO"
else
  spctl --assess --type open --context context:primary-signature -vv "$ALVO"
fi
