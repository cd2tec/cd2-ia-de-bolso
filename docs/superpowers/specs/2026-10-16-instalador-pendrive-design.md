# Instalador do pen drive (Windows e Mac): design

Data: 2026-10-16. Status: rascunho para aprovação.

## Por que

Hoje, para gravar o pen drive da IA de Bolso, a pessoa precisa baixar o `cd2tech-iadebolso.zip`, descompactar, dar duplo clique num `.bat` ou `.command`, ler uma lista de discos num terminal, escolher um pelo identificador (`/dev/disk5`, número de disco) e digitar `APAGAR`. Funciona bem para quem é técnico. Para quem é da área de negócio, que é o público que queremos que grave o próprio pen drive, cada um desses passos é um lugar para desistir.

O objetivo é que alguém da área de negócio consiga, sozinho, sair da página de download com um pen drive pronto, sem ver terminal e sem ter de decidir nada técnico.

## Objetivos

- Um instalador pequeno, baixado da página do projeto (`docs/index.html`, GitHub Pages), com uma versão para Windows e outra para Mac.
- Janela de verdade (desktop), no formato **passo a passo, uma pergunta por tela** (opção A dos rascunhos), com barra de etapas no topo.
- O instalador baixa sozinho tudo de que precisa (os arquivos da IA de Bolso, o motor, o modelo, o toolbox e o cd2-ctl). Não há ZIP para descompactar.
- O pen drive gerado é **idêntico** ao que os scripts `make-usb-*` geram hoje. Do `INICIAR` em diante, nada muda.

## Não objetivos

- Assinar o instalador (Apple Developer ID, notarização, certificado de código no Windows). Foi adiado de propósito. O aviso do sistema na primeira abertura continua aparecendo.
- Linux. Continua com `gravar-pendrive-linux.sh`.
- Mudar qualquer coisa na experiência de uso do pen drive (launchers, `web/`, `cd2-ctl`).
- Aposentar os scripts `GRAVAR-*` e `core/make-usb-*`. Eles ficam no repositório para quem prefere o jeito técnico e para gravação em série (`CD2_USB_DISK`, `CD2_USB_YES`).
- Escolher modelo, editar `web/config.json` ou qualquer personalização. O instalador grava o padrão.

## Antiobjetivos (seria um fracasso mesmo funcionando)

- Apagar um disco que não é pen drive. A regra de hoje vale e é inegociável: discos internos nunca aparecem na lista.
- A pessoa ver um terminal, um caminho de arquivo, um identificador de disco ou uma mensagem em inglês do sistema sem tradução.
- Ficar parado sem dizer nada. Toda espera longa mostra o que está acontecendo e quanto falta.
- Um pen drive que "parece pronto" mas não passou na conferência de checksums.

## O fluxo (decidido)

1. Na página do projeto, botões **Baixar para Windows** e **Baixar para Mac**. Vem um arquivo pequeno, só o instalador.
2. A pessoa abre o instalador. No Windows ele pede permissão de administrador ao abrir, uma vez só, porque formatar exige. No Mac não pede senha, porque apagar disco externo com `diskutil` não precisa.
3. **Tela 1, "Plugue o pen drive".** O instalador fica observando e mostra o pen drive assim que ele aparece, sem botão de atualizar. Com mais de um, a pessoa escolhe. Cada item mostra nome do modelo e tamanho em linguagem humana ("SanDisk 32 GB").
4. **Tela 2, "Confirme antes de apagar".** Mostra o nome, o tamanho e quantos arquivos há no pen drive. A pessoa marca "Pode apagar" e clica em **Gravar pen drive**. Isso substitui o "digite APAGAR".
5. **Tela 3, "Gravando".** Uma barra de progresso única para a gravação inteira, com uma frase simples da etapa atual ("Baixando o modelo (1,6 de 3,9 GB)", "Copiando para o pen drive", "Conferindo") e o tempo estimado que falta. Botão Cancelar.
6. **Tela 4, "Pronto, pode tirar o pen drive".** O instalador já ejetou. Botão **Gravar outro**, que volta para a Tela 1. Do segundo pen drive em diante nada é baixado, só copiado.

Rascunhos das telas: `.superpowers/brainstorm/` (fora do git).

## Como é feito (decidido)

- **Go + Wails.** O repositório já tem Go (`ctl/`) e já compila para várias plataformas. O Wails gera um `.exe` e um `.app` com janela nativa e telas em HTML/CSS/JS, o que deixa seguir o visual do `web/`.
- **A lógica dos `make-usb-*` é portada para Go dentro do instalador**, sem chamar os scripts por baixo: achar os pen drives, formatar em exFAT com MBR e nome `CD2`, copiar para a raiz, limpar `._*` e `.DS_Store`, conferir os checksums lendo do pen drive, guardar uma cópia do `CHECKSUMS.sha256` no cache e ejetar.
- O preparo (`prepare-*`: download com retomada, conferência de tamanho e SHA-256 do modelo, toolbox e cd2-ctl de versões fixas, geração do `CHECKSUMS.sha256`) também é portado.

## Comportamento que precisa ser igual ao de hoje

Os scripts atuais são a especificação. Em especial:

- **Quais discos aparecem.** Ponto de partida igual ao de hoje: no Mac, `diskutil list external physical`; no Windows, `Get-Disk` com `BusType` em USB, SD ou MMC. Nada fora disso. **O instalador é mais restritivo que o script (decisão do Gabriel, 16/10):** esconde sempre o disco de onde o sistema está ligando (no Windows, `IsBoot`/`IsSystem`), os discos com volume do Time Machine ou volume de sistema APFS e qualquer disco acima de 256 GB. Quem precisar gravar num disco que ficou de fora usa o script técnico.
- **Formatação.** exFAT, tabela MBR, rótulo `CD2`. No Windows: `Clear-Disk`, garantir MBR (o `Initialize-Disk` só aceita disco RAW), partição ativa ocupando tudo e `Format-Volume`. No Mac: `diskutil eraseDisk ExFAT CD2 MBR`.
- **Conteúdo e layout do pen drive.** O mesmo de `docs/criar-pendrive.md`, seção "O que deve aparecer no pen drive", com os mesmos itens excluídos (scripts de gravação, `docs/`, `ctl/`, `tests/`).
- **Cache.** As mesmas pastas de hoje (`%LOCALAPPDATA%\cd2-harness\usb`, `~/Library/Caches/cd2-harness/usb`), para que quem já gravou pelo script não baixe tudo de novo.
- **Download.** Retoma se cair e pula o que já está completo. Mesmas versões fixas do toolbox e do cd2-ctl, mesmo SHA-256 do modelo.
- **Conferência.** Checksum diferente no final significa falha, com a orientação "não use este pen drive, grave de novo".

## Distribuição

- Segue o padrão do `zip-iadebolso.yml`: a cada push na `main`, um workflow compila o instalador e publica na release fixa `iadebolso`, ao lado do ZIP. O link de download nunca muda e a release continua sem virar "latest", para não disputar com as do cd2-ctl.
- O instalador baixa os arquivos da IA de Bolso (`core/`, `web/`, launchers, `README.md`, `LICENSE`, `VERSION`) do `cd2tech-iadebolso.zip` dessa mesma release.
- `docs/index.html` troca o botão único "Baixar" por **Baixar para Windows** e **Baixar para Mac**, e mantém um link discreto para o ZIP e para o passo a passo técnico.

## Mensagens de erro

Sempre em português, curtas, dizendo o que fazer. Os casos de "Se der problema" em `docs/criar-pendrive.md` viram mensagens na tela: antivírus removendo o `llamafile.exe`, download interrompido, checksum diferente, pen drive pequeno demais, pen drive removido no meio.

## Calls que fiz (revisar)

- **Tamanho mínimo.** Bloquear abaixo de 4 GB, que é o que o script já faz e o que a gravação realmente exige. Abaixo de 16 GB, aviso recomendando um maior, mas deixo seguir. Na conversa eu tinha dito "menor que 16 GB não deixa seguir". Ajustei para não recusar um pen drive que funciona.
- **Formato do Mac:** `.dmg` com o `.app` dentro. Windows: um `.exe` único.
- **Pasta no repositório:** `installer/`, com `go.mod` próprio, como o `ctl/`.
- **Aviso do sistema na primeira abertura:** como não há assinatura, a página de download mostra logo abaixo do botão um quadro curto com o que fazer (Windows: "Mais informações" → "Executar assim mesmo"; Mac: Ajustes do Sistema → Privacidade e Segurança → "Abrir mesmo assim").

- **Versões vêm de um manifesto, não de constantes.** O mesmo workflow que compila o instalador e o ZIP publica na release `iadebolso` um `manifesto.json`, gerado a partir do mesmo commit, com a versão (`VERSION`), o SHA-256 do ZIP, a URL e o SHA-256 do modelo, as versões do toolbox e do cd2-ctl e a URL do llamafile já resolvida. O instalador lê o manifesto ao começar e confere o ZIP pelo SHA-256, não pelo tamanho. Assim, um instalador baixado há semanas grava o pen drive de hoje. O llamafile deixa de ser "o mais novo na hora" e passa a ser fixado no manifesto, o que é uma melhora em relação ao script.
- **Funciona sem internet quando a cache está completa.** Se a rede falhar, o instalador usa o último manifesto guardado na cache e grava, desde que tudo o que ele lista já esteja baixado e conferido. Sem cache completa e sem rede, diz isso com clareza e não formata nada. **Nada é apagado antes de todos os downloads terminarem e serem conferidos.**
- **Windows com outra conta autorizando o administrador.** Nesse caso raro, a cache passa a ser a do administrador e a primeira gravação baixa tudo de novo. Aceito isso para manter o "pede administrador ao abrir". Não vale a complexidade de separar a interface de um processo auxiliar elevado.

## Fica para quem construir

- Como empacotar e compilar no CI (runner de Mac e de Windows, WebView2 no Windows).
- Como detectar o pen drive sendo plugado (polling curto já resolve).
- Como estimar o tempo restante.
- Visual exato das telas, partindo dos rascunhos e do `web/`.
- Testes: a lógica de escolher disco, montar a cópia local e conferir checksums deve ser testável sem um pen drive de verdade.
