# IA de Bolso no Linux (AppImage)

No Linux a IA de Bolso funciona como no Windows e no Mac: o site oferece o instalador para baixar, o
instalador grava o pen drive, e no pen drive a pessoa dá dois cliques no lançador. Os dois programas são
AppImages (um arquivo só, sem instalar nada). Os itens marcados **(a confirmar)** são leituras nossas,
para o produto aprovar.

| Peça | Arquivo | Onde fica |
|---|---|---|
| Instalador (grava o pen drive) | `CD2-IA-de-Bolso-Linux.AppImage` | release `iadebolso`, botão "Baixar para Linux" do site |
| Lançador (abre a IA) | `IA de Bolso.AppImage` | raiz do pen drive, ao lado do `.exe` e do `.app` |

O pen drive é o mesmo para os três sistemas: um pen drive gravado no Windows ou no Mac abre no Linux, e o
contrário também.

## O que é diferente no Linux

- **Só pen drive.** No Linux não há a instalação no computador (a paga). O instalador mostra só "Criar o pen
  drive"; o lançador e a conversa não convidam a instalar. Como os agendamentos são da versão instalada,
  eles ficam indisponíveis no Linux, e o convite do agendamento explica isso em vez de oferecer a
  instalação **(a confirmar)**.
- **Cópia fora do pen drive.** Rodada de uma pasta do disco, a AppImage conta como cópia instalada sem chave
  e não sobe: a janelinha diz "Neste computador eu rodo só pelo pen drive" **(a confirmar)**.
- **Ícone na bandeja.** Pelo StatusNotifierItem (DBus), que o Ubuntu (extensão AppIndicator), o KDE e o XFCE
  mostram. No GNOME sem essa extensão (Debian, por exemplo) não há ícone: aí fechar a janelinha põe a IA para
  dormir, para ela não ficar escondida sem volta. Com ícone, fechar esconde, como no Windows, e a primeira vez
  avisa "Continuo acordada no ícone da barra de cima. Me chama quando quiser." **(a confirmar)**.
- **Conversa.** Abre como janela de aplicativo no Chrome, Edge, Chromium (inclusive o do snap) ou Brave; sem
  nenhum deles, no navegador padrão (`xdg-open`).
- **Motor e cd2-ctl.** Rodam de uma cópia em `~/.cache/cd2-harness`, como no Mac (o exFAT não guarda a
  permissão de execução), e o cd2-ctl é o `bin/ctl/linux-amd64`.
- **Plano B pelo terminal.** Com a AppImage no pen drive, o `ia-de-bolso.sh` vai para `core/` (a mesma regra do
  `.bat` e do `.command`): `sh core/ia-de-bolso.sh`.
- **Primeira abertura do instalador baixado.** O navegador salva o arquivo sem permissão de execução: botão
  direito, Propriedades, marcar "Permitir executar como programa". O site explica isso.

## Requisitos

- Linux 64 bits (x86_64), com área de trabalho: Ubuntu 22.04 ou mais novo, Debian 12 ou mais novo **(a
  confirmar a lista com o produto)**.
- WebKitGTK 4.1 e GTK 3 do sistema (`libwebkit2gtk-4.1-0`), que esses sistemas com área de trabalho já trazem.
  Não vão dentro da AppImage.
- A AppImage usa o runtime estático do AppImage: não precisa da `libfuse2`. Precisa só do FUSE do sistema
  (`fusermount3`), que vem no Ubuntu e no Debian com área de trabalho.
- Notificações: `notify-send` (pacote `libnotify-bin`) ou, sem ele, o `gdbus`.

## Como gerar

```sh
sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config
export APPIMAGETOOL=/caminho/appimagetool-x86_64.AppImage APPIMAGE_RUNTIME=/caminho/runtime-x86_64
sh launcher/build-linux.sh     # launcher/build/bin/IA de Bolso.AppImage
sh installer/build-linux.sh    # installer/build/bin/CD2-IA-de-Bolso-Linux.AppImage
```

O appimagetool 1.9.1 e o runtime 20251108 vêm de `github.com/AppImage`; o CI baixa os dois com o SHA-256
fixo em `.github/scripts/ferramentas-appimage.sh`. No CI (`zip-iadebolso.yml`), o job `lancador-linux`
gera o lançador, que entra no ZIP do pen drive; o `instalador-linux` gera o instalador e o publica na
release. Os dois compilam no Ubuntu 22.04, para rodar dali em diante.

## O que mudou no código

Tudo que é só do Linux está em arquivos `_linux` ou em ramos `goos == "linux"`. Nas partes comuns, as
mudanças só acrescentam o Linux; o comportamento do Windows e do Mac não muda.

| Onde | O quê |
|---|---|
| `launcher/bandeja_linux.go` | ícone na bandeja (fyne.io/systray, já usado no Windows e no Mac), conferência do ícone no DBus, aviso do primeiro fechar |
| `launcher/main.go` | raiz do pen drive pelo `$APPIMAGE`; ícone e nome da janela no Linux |
| `launcher/internal/raiz`, `ctl`, `sessao`, `navegador` | raiz da AppImage, `bin/ctl/linux-<arch>`, cópia do motor no cache, navegadores do Linux |
| `launcher/janela.go`, `instalada.go`, `bandeja.go`, `desinstalar.go`, `frontend/dist/tela.js` | sem convite para instalar onde não há instalação; X sem ícone dorme; tela "só pelo pen drive" |
| `instalacao/sistema` | `Instalavel(goos)`: Windows e Mac sim, Linux não |
| `ctl/internal/inst`, `web/instalar.js`, `web/index.html` | `/instalacao` diz `instalavel`; a conversa não convida a instalar no Linux |
| `installer/` | `PodeInstalarAqui`; a escolha só com o pen drive; a AppImage no ZIP, na raiz e no CHECKSUMS; ícone da janela |
| `core/lancadores.sh`, `core/make-usb-unix.sh`, `core/prepare-unix.sh`, `ia-de-bolso.sh` | a AppImage na gravação pelos scripts Unix; o `ia-de-bolso.sh` roda da raiz ou de `core/` |
| `launcher/build/linux/`, `launcher/build-linux.sh`, `installer/build-linux.sh` | montagem das AppImages |
| `.github/workflows/zip-iadebolso.yml`, `.github/scripts/ferramentas-appimage.sh` | jobs do Linux |
| `docs/index.html`, `README.md`, `docs/criar-pendrive.md` | botão "Baixar para Linux" e textos |

## Testado e pendente

Testado nesta máquina (Debian 13, KDE Plasma em Wayland):

- as duas AppImages montam e abrem com dois cliques;
- o instalador mostra só "Criar o pen drive";
- o lançador, numa pasta do disco, acha os arquivos e mostra a tela trancada;
- testes de Go e Node passam, com `go vet` para Windows e Mac.

Pendente:

- **Pen drive de verdade.** Gravar pelo instalador AppImage e abrir a `IA de Bolso.AppImage` do pen drive em
  exFAT. Confirmar que os dois cliques funcionam e que a IA acorda e abre a conversa.
- **Outras distribuições.** Ubuntu 22.04 e 24.04 (GNOME com AppIndicator; a restrição do AppArmor do 24.04
  à proteção da WebKitGTK) e Debian 12 com GNOME (sem ícone).
- **Scripts do Windows.** O `core/make-usb-windows.ps1` e o `core/prepare-windows.ps1` (plano B técnico do
  Windows) não levam a AppImage: não foram mexidos para não tocar no caminho do Windows. O instalador do
  Windows (`.exe`) já leva, porque a AppImage vem no ZIP.
- **ARM (`linux-arm64`).** Fora: o CI gera só x86_64.
