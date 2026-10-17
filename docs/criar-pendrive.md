# Como criar o pen drive da CD2 IA de Bolso

A ideia é simples: plugar um pen drive, dar um duplo clique e, alguns minutos depois, ter um pen drive que abre a IA da CD2 em qualquer computador, sem instalar nada e sem internet.

O caminho até aqui foi feito na mão uma vez, no Mac, e deu certo. Esta página guarda o que aprendemos e transforma tudo em um clique só.

## O que você precisa

- Um pen drive de **16 GB ou mais, USB 3.x**. O modelo tem 2,5 GB e é lido do pen drive toda vez que a IA de Bolso abre, então USB 2 deixa a partida bem mais lenta.
- Internet na primeira gravação, por uns **3,9 GB de download**. Os pen drives seguintes não baixam nada: o motor, o modelo e o toolbox ficam guardados no seu computador.
- Esta pasta, clonada do repositório ou baixada como ZIP.

Tudo que estiver no pen drive será apagado. Guarde o que importa antes.

## O jeito mais fácil: o instalador

Baixe o instalador na [página do projeto](https://cd2tec.github.io/cd2-ia-de-bolso/#pendrive) (`CD2-IA-de-Bolso-Windows.exe` ou `CD2-IA-de-Bolso-Mac.dmg`), abra e siga as quatro telas:

1. **Plugue o pen drive:** o instalador mostra o pen drive assim que ele aparece.
2. **Confirme antes de apagar:** marque "Pode apagar" e clique em Gravar pen drive.
3. **Gravando:** uma barra acompanha o download, a cópia e a conferência.
4. **Pronto, pode tirar o pen drive:** ele já foi ejetado. Clique em Gravar outro para o próximo.

O instalador esconde discos acima de 256 GB, discos de boot e discos do Time Machine, para ninguém apagar o que não devia. Se o seu pen drive cair nesses casos, o caminho é o script, descrito abaixo.

## O jeito de um clique

| Sistema | O que fazer |
|---|---|
| Windows | duplo clique em `GRAVAR-PENDRIVE-Windows.bat` (ele pede permissão de administrador, que é obrigatória para formatar) |
| Mac | duplo clique em `GRAVAR-PENDRIVE-Mac.command` |
| Linux | no terminal, dentro da pasta: `sh gravar-pendrive-linux.sh` |

O script mostra os pen drives encontrados, pede para você escolher um e só segue depois que você digita **APAGAR**. Discos internos nunca aparecem na lista.

Daí ele faz, nesta ordem:

1. **Monta uma cópia local** da IA de Bolso e baixa o llamafile e o modelo para uma pasta de cache do seu computador. Se a conexão cair, rode de novo: o download retoma de onde parou, e o que já estiver completo é pulado.
2. **Formata** o pen drive em exFAT com tabela MBR e nome `CD2`. É o formato que Windows, Mac e Linux leem e que aceita arquivo acima de 4 GB (FAT32 não aceita).
3. **Copia** tudo direto para a raiz do pen drive, com `bin/` e `models/` já preenchidos. Como o pen drive acabou de ser formatado, não precisa de subpasta: quem abre o pen drive já vê o `IA de Bolso`.
4. **Confere** os checksums lendo direto do pen drive, guarda uma cópia do `CHECKSUMS.sha256` fora dele e ejeta.

Pen drives gravados antes da v0.10.4 têm tudo dentro da pasta `CD2-HARNESS`. Eles continuam funcionando: é só abrir essa pasta e dar dois cliques no `INICIAR` (ou no `IA de Bolso`, se o pen drive já tiver o lançador). Para deixá-los no formato novo, com tudo na raiz, grave de novo.

No fim, a mensagem "Pronto. Pode tirar o pen drive." Para gravar o próximo, plugue e rode de novo. Leva só o tempo da cópia.

Onde fica o cache:

| Sistema | Pasta |
|---|---|
| Windows | `%LOCALAPPDATA%\cd2-harness\usb` |
| Mac | `~/Library/Caches/cd2-harness/usb` |
| Linux | `~/.cache/cd2-harness/usb` |

As cópias do checksum ficam na mesma pasta, uma por gravação, com data e hora no nome.

## O que deve aparecer no pen drive

```
CD2/   (a raiz do pen drive)
  IA de Bolso.exe        (o lançador do Windows)
  INICIAR-CD2-Windows.bat (alternativa no Windows, pelo terminal)
  IA de Bolso.app        (o lançador do Mac)
  iniciar-cd2-linux.sh
  README.md  LICENSE  VERSION
  CHECKSUMS.sha256
  bin/llamafile          ~350 MB
  bin/llamafile.exe      ~350 MB  (mesmo arquivo, com a extensão que o Windows exige)
  models/Qwen3-4B-Instruct-2507-Q4_K_M.gguf   ~2,5 GB
  bin/toolbox/windows-amd64/toolbox.exe   ~295 MB
  bin/toolbox/darwin-arm64/toolbox        ~156 MB
  bin/toolbox/darwin-amd64/toolbox        ~167 MB
  bin/toolbox/linux-amd64/toolbox         ~316 MB
  bin/ctl/windows-amd64/cd2-ctl.exe       ~7 MB
  bin/ctl/darwin-arm64/cd2-ctl            ~7 MB
  bin/ctl/darwin-amd64/cd2-ctl            ~7 MB
  bin/ctl/linux-amd64/cd2-ctl             ~7 MB
  core/toolbox/                           receitas por motor
  core/   web/           (em core/ fica também INICIAR-CD2-Mac.command, o plano B do Mac)
```

O código do lançador (`launcher/`) fica só no repositório, como `installer/`. O `INICIAR-CD2-Windows.bat` fica na raiz junto do `IA de Bolso.exe`, como alternativa enquanto o lançador não for testado num Windows de verdade. Se o lançador do Mac não estiver na cópia que você gravou, o `INICIAR-CD2-Mac.command` fica na raiz, no lugar dele.

Os scripts de gravação (`GRAVAR-*`, `gravar-*` e `core/make-usb-*`) e a pasta `docs/` ficam de fora de propósito. Quem recebe o pen drive só precisa ver o `IA de Bolso`.

## Teste de fumaça (faça no primeiro pen drive, antes de duplicar)

1. Tire e plugue de novo o pen drive.
2. Dê dois cliques em `IA de Bolso` (`.exe` no Windows, `.app` no Mac). No Linux, `sh iniciar-cd2-linux.sh`.
3. Espere a janela abrir e mande uma pergunta qualquer. Na primeira vez, o modelo leva de 20 a 90 segundos para carregar; o robô da janelinha mostra quanto do modelo já foi lido do pen drive.
4. Se for usar banco: Configurações, modo MCP Nativo para Banco de dados, preencha a conexão e clique em Salvar e conectar. A lista com motor, versão e tabelas tem de aparecer.
5. Clique em **Colocar a IA para dormir**, espere "Boa noite! Pode tirar o pen drive." e ejete com segurança.

Se passou, está aprovado. Grave os outros com o mesmo script ou copie todo o conteúdo da raiz de um pen drive para a raiz de outro já formatado em exFAT, e compare o `CHECKSUMS.sha256` no final.

## Se der problema

- **`llamafile.exe` sumiu logo depois do download (Windows).** Foi o antivírus. Libere uma exceção para a pasta de cache e rode de novo.
- **SmartScreen reclamou do `IA de Bolso.exe` (ou do `.bat`).** "Mais informações" e depois "Executar assim mesmo". Esse caminho e o WebView2 (que a janelinha usa no Windows) ainda não foram testados num Windows de verdade; se a janelinha não abrir, dê dois cliques no `INICIAR-CD2-Windows.bat`, ali mesmo na raiz, que faz o mesmo pelo terminal.
- **Mac: o `IA de Bolso.app` pediu confirmação para abrir.** Gravado pelo instalador ou pelo `GRAVAR-PENDRIVE-Mac.command`, ele não deve pedir aviso, porque a cópia não leva quarentena (até agora isso só foi conferido numa imagem de disco em exFAT, não num pen drive de verdade). Se pedir, ou se alguém copiou o `.app` à mão de um ZIP baixado e o Gatekeeper bloqueou, o caminho é Ajustes do Sistema, Privacidade e Segurança, e o botão "Abrir mesmo assim". O plano B (`core/INICIAR-CD2-Mac.command`) pede botão direito, Abrir.
- **Fechei a janelinha e a IA continua rodando.** É assim mesmo: ela fica no ícone perto do relógio (Windows) ou na barra de menus (Mac). Para encerrar, "Colocar a IA para dormir".
- **Linux não achou `mkfs.exfat`.** Instale o `exfatprogs` (`sudo apt install exfatprogs` ou `sudo dnf install exfatprogs`). O script aceita também o `exfat-utils` mais antigo.
- **No Windows aparecem arquivos `._alguma-coisa`.** É resíduo de cópia feita no Mac pelo Finder. O script já limpa isso; se copiou na mão, apague-os, são inofensivos.
- **O download parou no meio.** Rode o mesmo script de novo. Ele retoma.
- **Checksum diferente no final.** Não use esse pen drive. Formate e grave de novo; se repetir, troque o pen drive.
- **Pen drive lento.** O script já baixa no disco do computador e só copia o resultado, então o tempo que sobra é só o da cópia dos 3 GB.

## O jeito manual (o que fizemos na primeira vez)

Serve para entender o que o script faz, ou para quando ele não puder rodar.

1. Formatar em exFAT. No Mac: Utilitário de Disco, Apagar, formato ExFAT, esquema Registro de Inicialização Master (MBR). No Windows: botão direito no pen drive, Formatar, exFAT, nome `CD2`. Pelo terminal do Mac é uma linha só:

   ```bash
   diskutil eraseDisk ExFAT CD2 MBR /dev/diskN
   ```

   Confira o `N` com `diskutil list external physical` antes. Esse comando apaga o disco inteiro.

2. Copiar o conteúdo da pasta do repositório para a raiz do pen drive, sem a pasta `.git` (e sem `docs/`, `ctl/`, `tests/` e os `GRAVAR-*`, que só servem no repositório).
3. Rodar o preparo uma vez, com internet: `sh core/prepare-unix.sh` no Mac/Linux ou duplo clique em `core\prepare-windows.bat` no Windows. Se o pen drive for lento, prepare numa pasta do disco e copie depois.
4. Conferir: `bin/llamafile` e `bin/llamafile.exe` com uns 350 MB cada, modelo com uns 2,4 GB, os quatro `bin/toolbox/*/toolbox` e os quatro `bin/ctl/*/cd2-ctl` (ou só os da variável `CD2_TOOLBOX_PLATFORMS`) e `CHECKSUMS.sha256` gerado. Guardar uma cópia do checksum fora do pen drive.
5. No Mac, apagar os arquivos `._*` que o Finder deixa em exFAT:

   ```bash
   find /Volumes/CD2 -name '._*' -delete
   ```

6. Teste de fumaça, ejetar, duplicar.

## Antes de sair gravando dez

- Em `web/config.json`, ajuste o texto de abertura e as sugestões. Se for deixar o Archeo como padrão, coloque só a URL (`mcp.mode` igual a `"mcp"`). O login é feito pela conta de cada pessoa, no navegador, e não fica gravado. **Token nunca**, porque o pen drive vai circular.
- O modelo padrão vem de um repositório pessoal no Hugging Face. Antes de distribuir sob a marca CD2, confirme licença e origem ou troque pelo GGUF oficial da Qwen (variável `CD2_MODEL_URL` no preparo).
- A conexão com o banco não vai no pen drive: em cada computador ela é feita pela tela Configurações e fica guardada só nele. Leve a conta de leitura pronta.
- Grave um, teste em um notebook que não seja o seu, e só então duplique.
