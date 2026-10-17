# Instalador do pen drive: plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** um instalador desktop para Windows e Mac, baixado da página do projeto, que conduz uma pessoa da área de negócio em quatro telas até um pen drive idêntico ao que os scripts `make-usb-*` gravam hoje.

**Architecture:** app Go + Wails v2 em `installer/`. A lógica fica em pacotes Go puros, testáveis sem pen drive e sem cgo: `manifesto` (versões fixas publicadas na release), `baixar` (download com retomada e conferência), `preparo` (cópia local no cache e `CHECKSUMS.sha256`), `discos` (listar, filtrar, apagar, ejetar, por sistema), `pendrive` (copiar e conferir) e `gravar` (a ordem de tudo, o progresso e as mensagens). O `main` do Wails só liga esses pacotes às telas, feitas em HTML/JS puro sem build, com a máquina de estados das telas em `fluxo.js`, testada com `node --test`. O workflow `zip-iadebolso.yml` passa a publicar também `manifesto.json` e os dois instaladores na release fixa `iadebolso`.

**Tech Stack:** Go 1.23, Wails v2.10.1, `howett.net/plist` (ler `diskutil -plist`), PowerShell 5.1 (cmdlets de disco no Windows), HTML/CSS/JS puro, `node --test`, GitHub Actions (ubuntu, macos-14, windows-latest).

**Spec:** `docs/superpowers/specs/2026-10-16-instalador-pendrive-design.md`

## Global Constraints

- O pen drive gravado tem o mesmo conteúdo e layout que `core/make-usb-unix.sh` e `core/make-usb-windows.ps1` geram: exFAT, MBR, rótulo `CD2`, tudo na raiz, sem `core/make-usb-unix.sh` nem `core/make-usb-windows.ps1`, sem `._*` nem `.DS_Store`.
- Cache nas mesmas pastas do script: Windows `%LOCALAPPDATA%\cd2-harness\usb`, Mac `~/Library/Caches/cd2-harness/usb`. Cópia local em `<cache>/CD2-HARNESS`, com `bin/`, `models/` e `CHECKSUMS.sha256` preservados entre gravações.
- Discos: ponto de partida `diskutil list external physical` (Mac) e `Get-Disk` com `BusType` em `USB`, `SD`, `MMC` (Windows). Esconder sempre: disco de boot ou de sistema, disco com volume APFS de papel `System`, `Data` ou `Backup`, disco com `Backups.backupdb` na raiz de algum volume, disco acima de `LimiteBytes = 260_000_000_000` (aceita pen drive vendido como "256 GB", que reporta um pouco mais que 256e9).
- Tamanho: abaixo de `4_000_000_000` bytes o disco aparece, mas não pode ser escolhido. Abaixo de `16_000_000_000`, pode, com aviso.
- **Nada é apagado antes de todos os downloads terminarem e passarem na conferência.**
- No Mac, ler o `diskutil` sempre com `-plist`: a saída em texto muda com o idioma do sistema.
- O instalador só lê da rede o `manifesto.json`, o ZIP e os arquivos que o manifesto lista. Se o manifesto vier da cache, o instalador não acessa a rede.
- Textos de tela em português do Brasil, sem emoji e sem travessão, com o texto exato fixado neste plano quando houver.
- Pacotes em `installer/internal/...` não importam Wails e compilam com `CGO_ENABLED=0` em linux, darwin e windows.
- `go test ./...` em `installer/` (sem o pacote `main`) e `node --test` na raiz ficam verdes ao fim de cada tarefa. `gofmt -l .` vazio.
- Versão 0.13.0 em `VERSION`, nos `?v=` do `web/index.html` e em `web/mcp.js` (Tarefa 10).
- Toda mensagem de commit termina com a linha `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

- **Disco trocado entre a Tela 1 e o clique em Gravar.** No Windows, o número de disco é reaproveitado quando se tira um pen drive e se pluga outro. Antes de apagar, o instalador lista de novo e exige o mesmo `ID`, `Nome` e `Bytes`. Se algo mudou, recusa sem apagar (teste na Tarefa 7).
- **`ConvertTo-Json` com um disco só devolve um objeto, não uma lista.** O parser do Windows aceita as duas formas (teste na Tarefa 5).
- **Pen drive arrancado no meio da cópia.** A pessoa precisa ler a frase de "pen drive removido", não um erro genérico (teste na Tarefa 7).
- **Arquivo pela metade de outra versão na cache, com o mesmo nome.** O SHA não confere: o instalador apaga e baixa uma vez de novo do zero, sem entrar em laço (teste na Tarefa 2).
- **Cancelar.** Durante o download, nada é apagado e a frase diz isso. Depois de apagar, a frase diz que o pen drive ficou incompleto (teste na Tarefa 7).

---

## Estrutura de arquivos

```
installer/
  go.mod                         módulo github.com/cd2tec/cd2-ia-de-bolso/installer
  main.go  app.go                Wails: janela, métodos ligados às telas (Tarefa 8)
  wails.json                     sem frontend:install/build (HTML puro)
  build/windows/wails.exe.manifest   requireAdministrator (Tarefa 9)
  build/darwin/Info.plist        nome "CD2 IA de Bolso" (Tarefa 9)
  frontend/dist/index.html estilo.css fluxo.js tela.js   (Tarefa 8)
  cmd/manifesto/main.go          gera manifesto.json no CI (Tarefa 1)
  internal/manifesto/            Tarefa 1
  internal/baixar/               Tarefa 2
  internal/preparo/              Tarefa 3
  internal/discos/               Tarefa 4 (filtro) e Tarefa 5 (sistemas)
  internal/pendrive/             Tarefa 6
  internal/gravar/               Tarefa 7
tests/instalador-fluxo.test.js   Tarefa 8
.github/workflows/ci.yml         + testes do installer (Tarefa 1)
.github/workflows/zip-iadebolso.yml  + manifesto (Tarefa 1) + instaladores (Tarefa 9)
docs/index.html  docs/criar-pendrive.md  README.md   (Tarefa 10)
```

---

### Tarefa 1: Manifesto de versões

**Files:**
- Create: `installer/go.mod`, `installer/internal/manifesto/manifesto.go`, `installer/internal/manifesto/scripts.go`, `installer/cmd/manifesto/main.go`
- Test: `installer/internal/manifesto/manifesto_test.go`, `installer/internal/manifesto/scripts_test.go`
- Modify: `.github/workflows/ci.yml`, `.github/workflows/zip-iadebolso.yml`

**Interfaces:**
- Produces:
  ```go
  type Arquivo struct {
      URL    string `json:"url"`
      SHA256 string `json:"sha256,omitempty"`
      Bytes  int64  `json:"bytes,omitempty"`
  }
  type Manifesto struct {
      Versao     string   `json:"versao"`
      Zip        Arquivo  `json:"zip"`
      Llamafile  Arquivo  `json:"llamafile"`
      Modelo     Arquivo  `json:"modelo"`
      ModeloNome string   `json:"modelo_nome"`
      Toolbox    struct{ Versao, Base string; Plataformas []string } `json:"toolbox"`
      Ctl        struct{ Versao, Base string } `json:"ctl"`
  }
  func Ler(r io.Reader) (*Manifesto, error)   // erro se faltar Versao, Zip.URL, Zip.SHA256, Modelo.URL, ModeloNome, Toolbox.Versao, Ctl.Versao
  func Obter(ctx context.Context, c *http.Client, url, cache string) (m *Manifesto, daCache bool, err error)
  var ErrSemManifesto = errors.New("sem manifesto")
  type Fixos struct{ ModeloURL, ModeloNome, ModeloSHA256, LlamafileReserva, ToolboxVersao, ToolboxBase, CtlVersao, CtlBase string; Plataformas []string }
  func DosScripts(prepareUnix, prepareWindows string) (Fixos, error)
  ```
  Nomes JSON dos campos aninhados: `versao`, `base`, `plataformas`.

- [ ] **Step 1: Escrever os testes que falham**

  - `TestLerRecusaManifestoIncompleto`: `Ler` de `{"versao":"0.13.0"}` devolve erro que cita `zip`.
  - `TestObterGuardaNaCache`: servidor `httptest` devolve um manifesto válido. `Obter` devolve `daCache == false` e grava `<cache>/manifesto.json`.
  - `TestObterSemRedeUsaCache`: com o `manifesto.json` da cache gravado e o servidor fechado, devolve o mesmo manifesto com `daCache == true`.
  - `TestObterSemRedeSemCache`: devolve `ErrSemManifesto`.
  - `TestObterNaoAceitaHTTP404`: um 404 conta como "sem rede" e cai para a cache.
  - `TestDosScriptsLeOsValoresAtuais`: lê os arquivos reais `../../../core/prepare-unix.sh` e `../../../core/prepare-windows.ps1` e confere `ModeloNome == "Qwen3-4B-Instruct-2507-Q4_K_M.gguf"`, `ModeloSHA256 == "3605803b982cb64aead44f6c1b2ae36e3acdb41d8e46c8a94c6533bc4c67e597"`, `ToolboxVersao == "1.14.0"`, `CtlVersao == "0.6.0"`, `CtlBase == "https://github.com/cd2tec/cd2-ia-de-bolso/releases/download/ctl-v0.6.0"`, `Plataformas == []string{"windows-amd64","darwin-arm64","darwin-amd64","linux-amd64"}`.
  - `TestDosScriptsRecusaDivergencia`: se `prepare-windows.ps1` tiver uma versão de toolbox diferente da do `.sh` (texto alterado no teste), devolve erro que cita `toolbox`.

- [ ] **Step 2: Rodar e ver falhar.** `cd installer && go test ./internal/manifesto/` → FAIL (pacote não existe).

- [ ] **Step 3: Implementar.**

  - `DosScripts` lê os padrões `NOME="${CD2_X:-valor}"` do `.sh`. No caso do SHA do modelo, o valor é o do ramo `else`. Lê também os equivalentes do `.ps1`, e cada valor fixo precisa bater entre os dois arquivos.
  - `Obter` usa `http.Client` com prazo de 15 s. Qualquer erro de rede ou status diferente de 200 vira leitura da cache. Grava na cache com arquivo temporário e rename.
  - `cmd/manifesto` recebe as flags `-prepare-unix`, `-prepare-windows`, `-version-file`, `-zip <arquivo>`, `-zip-url`, `-llamafile-url`, `-llamafile-bytes`, `-modelo-bytes` e escreve o JSON em stdout. O SHA do ZIP é calculado do arquivo. O `Toolbox.Base` sai já com a versão expandida: `https://storage.googleapis.com/mcp-toolbox-for-databases/v1.14.0`.

- [ ] **Step 4: Rodar e ver passar.** `cd installer && go test ./internal/manifesto/` → PASS.

- [ ] **Step 5: Publicar no CI.**

  - Em `ci.yml`, um job `installer` igual ao do `ctl` (`go-version-file: installer/go.mod`) com `gofmt`, `go vet ./internal/... ./cmd/...`, `GOOS=windows go vet ./internal/... ./cmd/...`, `GOOS=darwin go vet ./internal/... ./cmd/...` e `go test ./internal/... ./cmd/...`.
  - Em `zip-iadebolso.yml`, depois de montar o ZIP:
    - resolver a URL do llamafile com a mesma consulta do `prepare-unix.sh` (API `releases/latest` do `mozilla-ai/llamafile`, reserva `FALLBACK_BIN_URL`);
    - ler o `content-length` do llamafile e do modelo com `curl -sIL`;
    - rodar `go run ./cmd/manifesto` com `-zip-url https://github.com/${{ github.repository }}/releases/download/iadebolso/cd2tech-iadebolso.zip`;
    - subir `manifesto.json` junto com o ZIP (`--clobber`).
  - O ZIP sobe antes do manifesto, para que um manifesto novo nunca aponte para o SHA de um ZIP que ainda não está lá.

- [ ] **Step 6: Commit.** `git add installer .github/workflows && git commit -m "Instalador: manifesto de versões publicado com o ZIP"`

---

### Tarefa 2: Download com retomada e conferência

**Files:**
- Create: `installer/internal/baixar/baixar.go`
- Test: `installer/internal/baixar/baixar_test.go`

**Interfaces:**
- Produces:
  ```go
  type Progresso func(feitos, total int64)
  var ErrConferencia = errors.New("arquivo baixado não confere")
  // Baixa url em destino. Pula se já estiver completo (SHA confere, ou Bytes bate quando não há SHA).
  // Retoma pela metade com Range; se o servidor ignorar o Range (200), recomeça do zero.
  // Se o SHA não conferir no fim, apaga e tenta UMA vez do zero; se falhar de novo, ErrConferencia.
  func Arquivo(ctx context.Context, c *http.Client, url, destino string, esperado manifesto.Arquivo, p Progresso) error
  // Só confere o que já está em disco, sem rede (modo cache).
  func Conferir(destino string, esperado manifesto.Arquivo) error
  ```

- [ ] **Step 1: Escrever os testes que falham** (servidor `httptest` com `http.ServeContent`, que já atende `Range`):
  - `TestBaixaInteiro`: arquivo de 1 MB, SHA certo → arquivo igual, `p` chamado com `total == 1<<20` no fim.
  - `TestRetomaPelaMetade`: destino com os primeiros 300 KB → o servidor recebe `Range: bytes=307200-` e o arquivo final confere.
  - `TestPulaSeCompleto`: destino completo e SHA certo → nenhuma requisição ao servidor.
  - `TestMetadeDeOutraVersao`: destino com 300 KB de conteúdo diferente → a primeira tentativa não confere, apaga e baixa do zero. Arquivo final certo, exatamente 2 GETs.
  - `TestServidorSempreErrado`: o servidor serve conteúdo com outro SHA → `ErrConferencia`, destino apagado, exatamente 2 GETs.
  - `TestSemSHAConfereTamanho`: `esperado.SHA256 == ""` e `Bytes` diferente do recebido → `ErrConferencia`.
  - `TestCancelar`: `ctx` cancelado no meio → `context.Canceled` e o arquivo parcial fica, para a retomada.
  - `TestConferirSemRede`: `Conferir` em arquivo ausente → erro que satisfaz `errors.Is(err, fs.ErrNotExist)`.

- [ ] **Step 2: Rodar e ver falhar.** `go test ./internal/baixar/` → FAIL.
- [ ] **Step 3: Implementar `Arquivo` e `Conferir`.** O SHA é calculado com `sha256.New()` sobre o arquivo inteiro depois de baixar. Mandar `User-Agent: cd2-harness`, como os scripts.
- [ ] **Step 4: Rodar e ver passar.** `go test ./internal/baixar/` → PASS.
- [ ] **Step 5: Commit.** `git commit -m "Instalador: download com retomada e conferência"`

---

### Tarefa 3: Cópia local no cache

**Files:**
- Create: `installer/internal/preparo/preparo.go`, `installer/internal/preparo/checksums.go`
- Test: `installer/internal/preparo/preparo_test.go`, `installer/internal/preparo/testdata/` (ZIP pequeno gerado no próprio teste com `archive/zip`)

**Interfaces:**
- Consumes: `manifesto.Manifesto`, `baixar.Arquivo`, `baixar.Conferir`
- Produces:
  ```go
  type Evento struct{ Item string; Feitos, Total int64 }   // Item: "zip", "motor", "modelo", "toolbox windows-amd64", "cd2-ctl darwin-arm64"...
  var ErrFaltaInternet = errors.New("falta baixar e não há internet")
  // Monta <cache>/CD2-HARNESS. Com semRede, só confere o que já existe (baixar.Conferir) e devolve ErrFaltaInternet se algo faltar.
  func Montar(ctx context.Context, c *http.Client, m *manifesto.Manifesto, cache string, semRede bool, ev func(Evento)) (stage string, err error)
  func BytesParaBaixar(m *manifesto.Manifesto, cache string) int64   // para o progresso
  func GerarChecksums(stage string) error                            // escreve CHECKSUMS.sha256
  func LerChecksums(caminho string) ([]Linha, error)
  type Linha struct{ SHA256, Caminho string }                        // Caminho sempre com "/"
  ```

- [ ] **Step 1: Escrever os testes que falham.** Usar um servidor `httptest` que serve um ZIP de teste, `SHA256SUMS` do ctl e arquivos falsos. O manifesto de teste rebaixa os mínimos de tamanho com a variável de pacote `minimos` (ver Step 3).
  - `TestMontarLayout`: depois de `Montar`, o stage tem `INICIAR-CD2-Windows.bat`, `INICIAR-CD2-Mac.command`, `iniciar-cd2-linux.sh`, `README.md`, `LICENSE`, `VERSION`, `web/`, `core/`, `bin/llamafile`, `bin/llamafile.exe` (bytes iguais aos de `bin/llamafile`), `models/<ModeloNome>`, `bin/toolbox/<plat>/toolbox[.exe]` e `bin/ctl/<plat>/cd2-ctl[.exe]` para as 4 plataformas, e `CHECKSUMS.sha256`. Não tem `core/make-usb-unix.sh`, `core/make-usb-windows.ps1`, `docs/`, `ctl/`, `tests/`, `GRAVAR-PENDRIVE-*`, `gravar-pendrive-linux.sh`, `.DS_Store` nem `._*`, mesmo que estejam no ZIP.
  - `TestMontarPreservaBinENoResto`: um arquivo velho `<stage>/antigo.txt` some, e `bin/llamafile` já existente e completo não é baixado de novo (contar GETs).
  - `TestMontarTiraModeloAntigo`: `models/velho.gguf` some.
  - `TestMontarModoExecutavel`: `iniciar-cd2-linux.sh` e `INICIAR-CD2-Mac.command` saem com bit de execução quando o ZIP traz o bit (teste pulado em Windows).
  - `TestCtlConfereComSHA256SUMS`: um `cd2-ctl` servido com conteúdo diferente do `SHA256SUMS` → erro que cita `cd2-ctl`. O `SHA256SUMS` fica guardado em `<cache>/ctl-SHA256SUMS-<versao>`.
  - `TestSemRedeComCacheCompleta`: depois de uma montagem completa, `Montar(..., semRede=true)` com o servidor fechado → OK.
  - `TestSemRedeFaltando`: com o modelo apagado → `ErrFaltaInternet`.
  - `TestChecksumsFormato`: o `CHECKSUMS.sha256` tem uma linha `<sha hex minúsculo>  <caminho com />` para `bin/llamafile`, `models/<ModeloNome>`, cada toolbox e cada cd2-ctl, em ordem alfabética de caminho, terminando com `\n`. `LerChecksums` lê esse formato e também o com `*` antes do caminho.

- [ ] **Step 2: Rodar e ver falhar.** `go test ./internal/preparo/` → FAIL.

- [ ] **Step 3: Implementar.**
  - O ZIP vai para `<cache>/cd2tech-iadebolso-<Versao>.zip` via `baixar.Arquivo`, conferido pelo `Zip.SHA256`.
  - Extrair só os itens da lista acima, tirando o prefixo `cd2tech-iadebolso/` e aplicando o `Mode()` do cabeçalho.
  - Mínimos de tamanho iguais aos dos scripts, em `var minimos = struct{ Modelo, Toolbox int64 }{1_000_000_000, 100_000_000}`, que os testes podem trocar.
  - Toolbox: `<Toolbox.Base>/<os>/<arch>/toolbox[.exe]`. cd2-ctl: `<Ctl.Base>/cd2-ctl-<plat>[.exe]`, conferido pelo `SHA256SUMS`, como no `prepare-unix.sh`.

- [ ] **Step 4: Rodar e ver passar.** `go test ./internal/preparo/` → PASS.
- [ ] **Step 5: Commit.** `git commit -m "Instalador: cópia local da IA de Bolso no cache"`

---

### Tarefa 4: Filtro de discos

**Files:**
- Create: `installer/internal/discos/discos.go`
- Test: `installer/internal/discos/filtro_test.go`

**Interfaces:**
- Produces:
  ```go
  const (MinimoBytes = 4_000_000_000; RecomendadoBytes = 16_000_000_000; LimiteBytes = 260_000_000_000)
  type Situacao string
  const (Ok Situacao = "ok"; Aviso Situacao = "aviso"; Pequeno Situacao = "pequeno")
  type Bruto struct {
      ID, Nome string; Bytes int64
      Boot, Sistema, TimeMachine bool
      Montagens []string          // pontos de montagem ou letras "E:\"
  }
  type Disco struct {
      ID, Nome string; Bytes int64
      Situacao Situacao
      Arquivos int; MaisDe bool     // MaisDe: parou de contar em 10.000
      Rotulo string                 // "SanDisk 32 GB"
  }
  func Filtrar(b []Bruto) []Bruto
  func Descrever(b Bruto, contar func(montagens []string) (int, bool)) Disco
  func ContarArquivos(montagens []string) (n int, maisDe bool)   // para em 10.000
  func Mesmo(a, b Disco) bool                                     // ID, Nome e Bytes iguais
  type Sistema interface {
      Listar(ctx context.Context) ([]Bruto, error)
      Apagar(ctx context.Context, id, rotulo string) (montagem string, err error)
      Ejetar(ctx context.Context, id, montagem string) error
  }
  ```

- [ ] **Step 1: Escrever os testes que falham.**
  - `TestFiltrarEscondeBootSistemaTimeMachine`: os três somem.
  - `TestFiltrarLimite`: `260_000_000_000` fica e `260_000_000_001` some.
  - `TestDescreverSituacao`: `3_999_999_999` → `Pequeno`; `4_000_000_000` → `Aviso`; `15_999_999_999` → `Aviso`; `16_000_000_000` → `Ok`.
  - `TestRotulo`: `Nome "SanDisk Ultra"`, `Bytes 32_017_047_552` → `"SanDisk Ultra 32 GB"`. Usar GB decimal arredondado, como na caixa do pen drive. Nome vazio → `"Pen drive 32 GB"`.
  - `TestContarArquivosPara`: pasta temporária com 10.005 arquivos → `(10000, true)`. Pasta vazia → `(0, false)`.
  - `TestMesmo`: o mesmo ID com `Bytes` diferente → `false`.
- [ ] **Step 2: Rodar e ver falhar.** `go test ./internal/discos/` → FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Rodar e ver passar.**
- [ ] **Step 5: Commit.** `git commit -m "Instalador: filtro de discos mais restrito que o script"`

---

### Tarefa 5: Discos no Mac e no Windows

**Files:**
- Create: `installer/internal/discos/executor.go`, `installer/internal/discos/mac.go`, `installer/internal/discos/windows.go`, `installer/internal/discos/listar.ps1`, `installer/internal/discos/apagar.ps1`, `installer/internal/discos/ejetar.ps1`, `installer/internal/discos/sistema_darwin.go`, `installer/internal/discos/sistema_windows.go`, `installer/internal/discos/sistema_outros.go`
- Test: `installer/internal/discos/mac_test.go`, `installer/internal/discos/windows_test.go`, `installer/internal/discos/testdata/*.plist`, `installer/internal/discos/testdata/*.json`

**Interfaces:**
- Consumes: `Bruto`, `Sistema` (Tarefa 4)
- Produces:
  ```go
  type Executor func(ctx context.Context, nome string, args ...string) ([]byte, error)
  func NovoMac(x Executor) Sistema
  func NovoWindows(x Executor) Sistema
  func Padrao() Sistema       // por build tag: darwin → NovoMac(exec real), windows → NovoWindows, outros → erro "sistema não suportado"
  // funções puras de leitura, testadas com fixtures:
  func lerListaMac(plist []byte) ([]string, error)                       // WholeDisks
  func lerInfoMac(plist []byte) (nome string, bytes int64, montagem string, err error)
  func lerBootMac(plist []byte) (string, error)                          // ParentWholeDisk de "diskutil info -plist /"
  func lerApfsMac(plist []byte) (map[string][]string, error)             // disco inteiro → papéis
  func lerDiscosWindows(js []byte) ([]Bruto, error)
  ```
  Sem a tag de sistema, os arquivos `mac.go` e `windows.go` compilam em qualquer sistema. Só `sistema_*.go` usa build tags.

- [ ] **Step 1: Escrever os testes que falham.**
  - Mac, com fixtures capturadas de `diskutil list -plist external physical`, `diskutil info -plist disk4`, `diskutil info -plist /` e `diskutil apfs list -plist`:
    - `TestMacListaWholeDisks`: lê `["disk4","disk5"]`.
    - `TestMacPapeisAPFS`: o `PhysicalStores` `disk5s2` com volume de papel `Backup` → `map["disk5"]` contém `"Backup"`.
    - `TestMacListarMarcaBootETimeMachine`: com um `Executor` falso que devolve as fixtures por argumento, `Listar` marca `Boot` no disco do `/`, `TimeMachine` no disco com papel `Backup` e `TimeMachine` quando há `Backups.backupdb` na montagem (pasta temporária).
    - `TestMacApagarComandos`: o `Executor` falso registra as chamadas, na ordem: `diskutil unmountDisk force /dev/disk4`, `diskutil eraseDisk ExFAT CD2 MBR /dev/disk4`, `diskutil info -plist /dev/disk4s1`. A montagem devolvida é o `MountPoint` lido do último plist.
    - `TestMacEjetar`: `diskutil eject /dev/disk4`.
  - Windows:
    - `TestWindowsUmDiscoObjeto`: JSON de objeto único (`{"Number":2,...}`) → um `Bruto`.
    - `TestWindowsLista`: JSON de lista com dois discos; um com `IsBoot: true` → `Boot == true`; `DriveLetters ["E"]` → `Montagens ["E:\\"]`; `ID == "2"`.
    - `TestWindowsApagarChamaPowerShell`: o `Executor` recebe `powershell -NoProfile -ExecutionPolicy Bypass -Command <apagar.ps1 embutido>` com `-DiskNumber 2 -Label CD2`, e a letra lida da saída (`E`) vira `E:\`.
- [ ] **Step 2: Rodar e ver falhar.** `go test ./internal/discos/` → FAIL.
- [ ] **Step 3: Implementar.**
  - `listar.ps1`: `Get-Disk | Where-Object BusType -in 'USB','SD','MMC'`. Selecionar `Number`, `FriendlyName`, `Size`, `IsBoot`, `IsSystem`, `BusType` e as letras de `Get-Partition`. Sair com `ConvertTo-Json -InputObject @(...)`, no PowerShell 5.1.
  - `apagar.ps1`: porta as linhas 80 a 104 de `core/make-usb-windows.ps1`: `Clear-Disk -RemoveData -RemoveOEM`, garantir MBR (`Initialize-Disk` só em RAW), `New-Partition -UseMaximumSize -IsActive -AssignDriveLetter`, `Format-Volume exFAT` e espera pela letra. Imprime só a letra.
  - `ejetar.ps1`: o `Shell.Application ... InvokeVerb('Eject')` da linha 132.
  - Os `.ps1` são embutidos com `//go:embed`.
- [ ] **Step 4: Rodar e ver passar.** `go test ./internal/discos/`, `GOOS=windows go vet ./internal/discos/` e `GOOS=darwin go vet ./internal/discos/` → PASS.
- [ ] **Step 5: Teste manual no Mac** com um pen drive descartável: um programa de teste em `installer/cmd/discos-teste/` (não publicado) que lista e imprime. Conferir que o SSD interno e um HD externo acima de 260 GB não aparecem.
- [ ] **Step 6: Commit.** `git commit -m "Instalador: listar, apagar e ejetar pen drive no Mac e no Windows"`

---

### Tarefa 6: Copiar e conferir no pen drive

**Files:**
- Create: `installer/internal/pendrive/pendrive.go`
- Test: `installer/internal/pendrive/pendrive_test.go`

**Interfaces:**
- Consumes: `preparo.LerChecksums`, `preparo.Linha`
- Produces:
  ```go
  var ErrRemovido = errors.New("pen drive removido")
  var ErrDiferente = errors.New("checksum diferente no pen drive")
  func TamanhoTotal(stage string) (int64, error)
  func Copiar(ctx context.Context, stage, destino string, p func(feitos, total int64)) error   // limpa ._* e .DS_Store no fim, faz Sync de cada arquivo
  func Conferir(ctx context.Context, destino string, p func(feitos, total int64)) error         // lê destino/CHECKSUMS.sha256
  func GuardarChecksums(destino, cache string, agora time.Time) (string, error)                 // <cache>/CHECKSUMS-20261016-143005.sha256
  ```
- [ ] **Step 1: Escrever os testes que falham.**
  - `TestCopiarArvore`: a árvore de pastas temporárias copiada fica idêntica, e `._x` e `.DS_Store` não chegam ao destino.
  - `TestCopiarDestinoSumiu`: o destino é apagado no meio, por um `p` que remove a pasta na primeira chamada → `errors.Is(err, ErrRemovido)`.
  - `TestConferirOk` e `TestConferirDiferente`: um byte alterado em `models/x.gguf` → `ErrDiferente`, com o caminho na mensagem.
  - `TestConferirSemChecksums`: sem `CHECKSUMS.sha256` → `ErrDiferente`.
  - `TestGuardarChecksumsNome`: com `agora = 2026-10-16 14:30:05` → `CHECKSUMS-20261016-143005.sha256`.
- [ ] **Step 2: Rodar e ver falhar.** `go test ./internal/pendrive/` → FAIL.
- [ ] **Step 3: Implementar.** Um erro de escrita vira `ErrRemovido` quando o `destino` não existe mais no momento do erro.
- [ ] **Step 4: Rodar e ver passar.**
- [ ] **Step 5: Commit.** `git commit -m "Instalador: copiar e conferir no pen drive"`

---

### Tarefa 7: A gravação, do começo ao fim

**Files:**
- Create: `installer/internal/gravar/gravar.go`, `installer/internal/gravar/mensagens.go`
- Test: `installer/internal/gravar/gravar_test.go`

**Interfaces:**
- Consumes: `manifesto.Obter`, `preparo.Montar`, `preparo.BytesParaBaixar`, `discos.Sistema`, `discos.Filtrar`, `discos.Descrever`, `discos.Mesmo`, `pendrive.*`
- Produces:
  ```go
  type Evento struct {
      Etapa  string  `json:"etapa"`   // "baixando" | "copiando" | "conferindo" | "pronto"
      Frase  string  `json:"frase"`
      Fracao float64 `json:"fracao"`  // 0..1 da gravação inteira
      Restante int   `json:"restante"`// segundos estimados; -1 se ainda não dá para estimar
  }
  type Erro struct{ Frase string; Causa error }   // Error() devolve Frase
  type Opcoes struct {
      Disco discos.Disco; Cache, URLManifesto string
      HTTP *http.Client; Sistema discos.Sistema; Agora func() time.Time
  }
  func Gravar(ctx context.Context, o Opcoes, ev func(Evento)) error   // erros sempre *Erro
  func PastaCache() (string, error)   // %LOCALAPPDATA%\cd2-harness\usb ou ~/Library/Caches/cd2-harness/usb
  func Discos(ctx context.Context, s discos.Sistema) ([]discos.Disco, error)   // Listar → Filtrar → Descrever
  const URLManifesto = "https://github.com/cd2tec/cd2-ia-de-bolso/releases/download/iadebolso/manifesto.json"
  ```
- Ordem fixa: `Obter` → `Montar(semRede = daCache)` → listar de novo e exigir `Mesmo` → `Apagar` → `Copiar` → `Conferir` → `GuardarChecksums` → `Ejetar` → evento `pronto`.
- Frases (texto exato, em `mensagens.go`):

  | Situação | Frase |
  |---|---|
  | baixando | `Baixando %s (%s de %s)` com item ("o modelo", "o motor", "o toolbox", "os arquivos da IA de Bolso") e GB com uma casa e vírgula |
  | copiando | `Copiando para o pen drive` |
  | conferindo | `Conferindo o que foi gravado` |
  | `ErrSemManifesto` ou `ErrFaltaInternet` | `Não consegui baixar o que falta. Confira a internet e tente de novo. Nada foi apagado.` |
  | `baixar.ErrConferencia` | `Um arquivo chegou com defeito duas vezes. Tente de novo daqui a pouco. Nada foi apagado.` |
  | `llamafile.exe` sumiu ou zerou logo depois do download (Windows) | `O antivírus removeu o motor da IA. Libere a pasta %s no antivírus e tente de novo. Nada foi apagado.` |
  | disco mudou antes de apagar | `O pen drive mudou desde que você escolheu. Escolha de novo. Nada foi apagado.` |
  | falha ao apagar | `Não consegui formatar o pen drive. Tire, plugue de novo e tente outra vez.` |
  | `pendrive.ErrRemovido` | `O pen drive foi removido no meio da gravação. Plugue de novo e grave outra vez.` |
  | `pendrive.ErrDiferente` | `A conferência encontrou diferença. Não use este pen drive. Grave de novo e, se repetir, troque de pen drive.` |
  | cancelado antes de apagar | `Gravação cancelada. Nada foi apagado.` |
  | cancelado depois de apagar | `Gravação cancelada. O pen drive ficou incompleto. Grave de novo antes de usar.` |
  | falha ao ejetar | não é erro: `pronto` com a frase `Pronto. Ejete o pen drive pelo sistema antes de tirar.` |

- [ ] **Step 1: Escrever os testes que falham.** Usar um `Sistema` falso, que registra as chamadas e devolve uma montagem em pasta temporária, e um servidor `httptest` com o manifesto e os arquivos da Tarefa 3.
  - `TestOrdem`: as chamadas ao `Sistema` são `Listar`, `Apagar`, `Ejetar`, nessa ordem, e o `Apagar` só acontece depois de o servidor ter atendido o último download.
  - `TestDownloadFalhaNaoApaga`: o servidor devolve 500 para o modelo → `*Erro` com a frase de internet e nenhum `Apagar`.
  - `TestDiscoMudou`: o segundo `Listar` devolve o mesmo ID com outro `Bytes` → frase "O pen drive mudou..." e nenhum `Apagar`.
  - `TestRemovidoNoMeio`: o `Sistema` falso apaga a montagem durante a cópia → frase de removido.
  - `TestCancelarAntesEDepois`: cancelar durante o download dá a frase "Nada foi apagado". Cancelar no primeiro evento `copiando` dá a frase "ficou incompleto".
  - `TestFracaoCresce`: a `Fracao` dos eventos nunca diminui e o último evento é `pronto` com `1`.
  - `TestSemRedeComCache`: com o servidor fechado depois de uma primeira gravação → segunda gravação OK, sem nenhuma requisição.
  - `TestFrases`: tabela com cada erro de origem → frase exata da tabela acima.
- [ ] **Step 2: Rodar e ver falhar.** `go test ./internal/gravar/` → FAIL.
- [ ] **Step 3: Implementar.** A fração pondera bytes: o que falta baixar, mais o tamanho total da cópia, mais o mesmo total de novo para a conferência. O tempo restante fica a critério de quem implementa: média móvel da vazão da etapa atual, `-1` nos primeiros 3 s.
- [ ] **Step 4: Rodar e ver passar.**
- [ ] **Step 5: Commit.** `git commit -m "Instalador: gravação completa com progresso e mensagens"`

---

### Tarefa 8: Janela e telas

**Files:**
- Create: `installer/main.go`, `installer/app.go`, `installer/wails.json`, `installer/frontend/dist/index.html`, `installer/frontend/dist/estilo.css`, `installer/frontend/dist/fluxo.js`, `installer/frontend/dist/tela.js`
- Test: `tests/instalador-fluxo.test.js`

**Interfaces:**
- Consumes: `gravar.Discos`, `gravar.Gravar`, `gravar.PastaCache`, `gravar.URLManifesto`, `discos.Padrao`
- Produces (métodos Wails em `App`, chamados por `window.go.main.App.*`):
  ```go
  func (a *App) ListarDiscos() ([]discos.Disco, error)
  func (a *App) Gravar(d discos.Disco) error   // devolve logo; emite "progresso" (gravar.Evento) e "fim" ({ok bool, frase string})
  func (a *App) Cancelar()
  ```
  Em `fluxo.js` (padrão UMD do projeto, `root.CD2Fluxo`):
  ```js
  inicial() -> { tela:'plugue', discos:[], escolhido:null, podeApagar:false, progresso:null, frase:'' }
  passo(estado, acao) -> estado   // acoes: {tipo:'discos', lista}, {tipo:'escolher', id}, {tipo:'continuar'},
                                  // {tipo:'marcar', valor}, {tipo:'gravar'}, {tipo:'progresso', evento},
                                  // {tipo:'fim', ok, frase}, {tipo:'outro'}, {tipo:'voltar'}
  podeContinuar(estado) -> bool
  ```
- Textos das telas (exatos):
  - Barra: `1 de 4` ... `4 de 4`.
  - Plugue: título `Plugue o pen drive`, texto `Use um de 16 GB ou mais. Tudo que estiver nele será apagado.`, sem pen drive `Procurando pen drive...`, pequeno `Pequeno demais: precisa de pelo menos 4 GB.`, aviso `Funciona, mas um de 16 GB ou mais é melhor.`, botão `Continuar`.
  - Confirmar: título `Confirme antes de apagar`, texto `O pen drive <b>%rotulo</b> tem %n arquivos. Eles vão sumir.` (`está vazio.` com zero, `mais de 10.000 arquivos` com `MaisDe`), caixa `Pode apagar`, botões `Voltar` e `Gravar pen drive`.
  - Gravando: título `Gravando... %p%`, a `frase` do evento, `Faltam uns %m minutos.` (oculto com `-1`; `Falta menos de um minuto.` abaixo de 60 s), texto `Pode deixar rodando.`, botão `Cancelar`.
  - Pronto: título `Pronto, pode tirar o pen drive.` (ou a frase do evento de ejetar), botão `Gravar outro`.
  - Erro: título `Não deu certo`, a frase do `fim`, botão `Tentar de novo` (volta para Plugue).

- [ ] **Step 1: Escrever os testes que falham** em `tests/instalador-fluxo.test.js`, com `require('../installer/frontend/dist/fluxo.js')`:
  - sem discos → `podeContinuar` falso;
  - um disco `ok` → já vem escolhido;
  - dois discos → nenhum escolhido até `escolher`;
  - disco `pequeno` → `escolher` não muda o estado;
  - o disco escolhido some da lista → `escolhido` volta a `null`;
  - em `confirmar`, `gravar` sem `marcar true` não muda a tela;
  - `voltar` em `confirmar` → `plugue` com `podeApagar` falso;
  - `fim ok:false` → tela `erro` com a frase;
  - `outro` em `pronto` → igual a `inicial()`;
  - durante `gravando`, uma ação `discos` não muda a tela.
- [ ] **Step 2: Rodar e ver falhar.** `node --test tests/instalador-fluxo.test.js` → FAIL.
- [ ] **Step 3: Implementar `fluxo.js`.**
- [ ] **Step 4: Rodar e ver passar.** `node --test` → PASS.
- [ ] **Step 5: Ligar as telas.**
  - `tela.js` desenha a partir do estado e chama `ListarDiscos` a cada 1,5 s só na tela `plugue`. Assina `progresso` e `fim` com `window.runtime.EventsOn`.
  - Visual a partir dos rascunhos e das cores e fontes do `web/`, com janela fixa de 560 × 420.
  - `main.go`: título `CD2 IA de Bolso`, sem redimensionar, `embed` de `frontend/dist`.
  - `wails.json` com `frontend:install` e `frontend:build` vazios.
- [ ] **Step 6: Rodar no Mac** com `cd installer && wails dev` e um pen drive descartável. Percorrer as quatro telas, gravar e abrir o pen drive com `INICIAR-CD2-Mac.command`.
- [ ] **Step 7: Commit.** `git commit -m "Instalador: janela com as quatro telas"`

---

### Tarefa 9: Empacotar e publicar

**Files:**
- Create: `installer/build/windows/wails.exe.manifest`, `installer/build/darwin/Info.plist`, `installer/build/appicon.png`
- Modify: `.github/workflows/zip-iadebolso.yml`

- [ ] **Step 1: Configurar o empacotamento.**
  - `wails.exe.manifest` com `<requestedExecutionLevel level="requireAdministrator" uiAccess="false"/>`.
  - `Info.plist` com `CFBundleName` e `CFBundleDisplayName` iguais a `CD2 IA de Bolso` e `CFBundleIdentifier` igual a `br.com.cd2.iadebolso.instalador`.
  - Ícone a partir do logo de `docs/assets`.
- [ ] **Step 2: Jobs no workflow.**
  - Dois jobs, `instalador-mac` (`macos-14`) e `instalador-windows` (`windows-latest`), com `needs: zip`.
  - Cada job roda `go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.1` e `wails build -clean -trimpath -ldflags "-X main.versao=<VERSION>"`. Mac com `-platform darwin/universal`, seguido de `codesign --force --deep -s -` no `.app` (assinatura ad hoc, que o Apple Silicon exige) e `hdiutil create -volname "CD2 IA de Bolso" -srcfolder <app> -ov -format UDZO CD2-IA-de-Bolso-Mac.dmg`. Windows com `-platform windows/amd64 -webview2 embed`, renomeado para `CD2-IA-de-Bolso-Windows.exe`.
  - Os dois sobem para a release `iadebolso` com `gh release upload --clobber`.
- [ ] **Step 3: Verificar.** Rodar o workflow por `workflow_dispatch` no branch e baixar os dois artefatos. No Windows, abrir e ver o pedido de administrador. No Mac, abrir pelo caminho "Abrir mesmo assim". Nos dois, chegar até a Tela 1.
- [ ] **Step 4: Commit.** `git commit -m "Instalador: empacotamento para Windows e Mac na release iadebolso"`

---

### Tarefa 10: Página, documentação e versão

**Files:**
- Modify: `docs/index.html`, `docs/criar-pendrive.md`, `README.md`, `VERSION`, `web/index.html`, `web/mcp.js`, `.github/workflows/zip-iadebolso.yml` (texto das notas da release)

- [ ] **Step 1: Página de download.**
  - Os botões "Baixar" de `docs/index.html` (linhas 208, 224 e 247) viram dois: `Baixar para Windows`, que leva a `.../releases/download/iadebolso/CD2-IA-de-Bolso-Windows.exe`, e `Baixar para Mac`, que leva a `.../CD2-IA-de-Bolso-Mac.dmg`.
  - Abaixo deles, um quadro curto. Windows: `Se aparecer "O Windows protegeu o computador", clique em Mais informações e depois em Executar assim mesmo.` Mac: `Se o Mac não deixar abrir, vá em Ajustes do Sistema, Privacidade e Segurança, e clique em Abrir mesmo assim.`
  - Um link discreto: `Prefere o jeito técnico? Baixe o ZIP com os scripts.`
- [ ] **Step 2: Documentação.**
  - `docs/criar-pendrive.md` ganha, antes de "O jeito de um clique", a seção `## O jeito mais fácil: o instalador`, com as quatro telas em uma linha cada. Uma nota diz que o instalador esconde discos acima de 256 GB, de boot e do Time Machine, e que, para esses casos, o caminho é o script.
  - `README.md` aponta para a mesma seção.
  - As notas da release em `zip-iadebolso.yml` passam a citar o instalador primeiro.
- [ ] **Step 3: Versão 0.13.0** em `VERSION`, em todos os `?v=` de `web/index.html` e no `version:'...'` de `web/mcp.js`.
- [ ] **Step 4: Rodar tudo.** `node --test` na raiz e `cd installer && go test ./internal/... ./cmd/...` → PASS.
- [ ] **Step 5: Commit.** `git commit -m "Página com download do instalador; v0.13.0"`
