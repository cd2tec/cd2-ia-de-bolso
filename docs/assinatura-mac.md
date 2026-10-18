# Assinatura e notarização no Mac

O workflow `zip iadebolso` assina o lançador (`IA de Bolso.app`, que vai no ZIP) e o instalador (`CD2 IA de Bolso.app` e o `CD2-IA-de-Bolso-Mac.dmg`) com o certificado **Developer ID Application** da CD2 (Team ID `M8HDQJFJAS`) e manda tudo para a notarização da Apple. O ticket fica grampeado no `.app` e no `.dmg`, então o Gatekeeper aceita sem aviso e sem internet. Quem faz isso é o [`.github/scripts/assinar-mac.sh`](../.github/scripts/assinar-mac.sh).

Sem os secrets abaixo, o workflow não quebra: assina ad hoc, como antes, e deixa um aviso no resumo da execução. Secret preenchido pela metade é erro, de propósito.

## Secrets do repositório

Settings → Secrets and variables → Actions → New repository secret.

| Secret | O que é | Como gerar |
| --- | --- | --- |
| `MAC_CERT_P12_BASE64` | O certificado Developer ID Application com a chave privada, em base64 | Acesso às Chaves → Meus Certificados → botão direito no certificado → Exportar → `.p12`. Depois `base64 -i cd2-developer-id.p12 \| pbcopy` |
| `MAC_CERT_PASSWORD` | A senha definida na exportação do `.p12` | |
| `ASC_KEY_P8_BASE64` | Chave da App Store Connect API, em base64 | App Store Connect → Users and Access → Integrations → Team Keys, papel Developer. `base64 -i AuthKey_XXXX.p8 \| pbcopy` |
| `ASC_KEY_ID` | O Key ID dessa chave (10 caracteres) | Aparece na lista de chaves |
| `ASC_ISSUER_ID` | O Issuer ID (UUID) | Aparece no topo da página de chaves |

O `.p12` e o `.p8` são as chaves para assinar em nome da CD2. Guarde-os num cofre, nunca em chat ou e-mail.

## Rodar no próprio Mac

Com o certificado instalado e um perfil salvo por `xcrun notarytool store-credentials cd2-notary --key AuthKey_XXXX.p8 --key-id ... --issuer ...`:

```bash
MAC_IDENTIDADE="Developer ID Application: CD2 Servicos em Tecnologia da Informacao Ltda (M8HDQJFJAS)" MAC_NOTARY_PROFILE=cd2-notary bash .github/scripts/assinar-mac.sh app "launcher/build/bin/IA de Bolso.app"
```

## Se der problema

- **`unable to build chain to self-signed root` e `Unknown format in import`.** Ou falta o intermediário da Apple (instale o `DeveloperIDG2CA.cer` de apple.com/certificateauthority), ou o certificado está com "Confiar sempre" no Acesso às Chaves. Volte para "Usar Padrões do Sistema".
- **`security find-identity -v -p codesigning` mostra 0 identidades.** O certificado foi instalado sem a chave privada (CSR gerado em outro Mac) ou falta o intermediário.
- **Notarização `Invalid`.** O script imprime o log da Apple com o motivo. Hardened Runtime e timestamp já vão ligados.
- **O certificado vence** em 17/11/2027. Gere outro Developer ID Application (Sub-CA G2), exporte o `.p12` e troque os dois primeiros secrets.

## O que a notarização não cobre

Só o que está dentro do `.app`. O `llamafile` e os outros binários do pen drive ficam fora: abrem sem aviso quando gravados pelo instalador ou pelo `GRAVAR-PENDRIVE-Mac.command`, que não levam quarentena, mas seriam bloqueados se alguém extraísse o ZIP baixado pelo navegador à mão.
