# Lançador do pen drive Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Trocar o `INICIAR-CD2-Windows.bat` e o `INICIAR-CD2-Mac.command` da raiz do pen drive por `IA de Bolso.exe` e `IA de Bolso.app`: uma janelinha em pixel art que carrega a IA, abre a conversa no navegador e fica num ícone perto do relógio ou na barra de menus.

**Architecture:** Novo módulo Go `launcher/`. O núcleo (`launcher/internal/...`) porta a lógica de `core/start-windows.ps1` e `core/start-unix.sh` em pacotes pequenos, sem nenhuma dependência de interface gráfica e testados com processos falsos. A casca (`launcher/main.go`, `launcher/janela*.go`) liga esse núcleo a uma janela HTML e a um ícone de bandeja, com a biblioteca escolhida no spike da Task 0. O lançador entra no ZIP pelo CI, e a gravação (scripts e instalador) põe na raiz o lançador de cada plataforma. Se ele faltar, a gravação põe o `.bat` ou o `.command` no lugar.

**Tech Stack:** Go 1.23. Wails v2.10.1 (o mesmo do instalador) ou Wails v3, conforme o spike. `fyne.io/systray` se o spike ficar com o Wails v2. HTML, CSS e JS puros na janelinha. `node --test` para os testes de front.

**Spec:** `docs/superpowers/specs/2026-10-17-lancador-pendrive-design.md`

## Global Constraints

- Nomes na raiz do pen drive: `IA de Bolso.exe` (Windows) e `IA de Bolso.app` (Mac, universal Intel e Apple Silicon, com assinatura ad hoc). O executável dentro do `.app` é `IA de Bolso.app/Contents/MacOS/IA de Bolso`.
- Nenhum terminal aparece: no Windows, o `.exe` é GUI, e o motor e o `cd2-ctl` sobem com a janela escondida.
- O `.exe` roda como o usuário comum (manifesto `asInvoker`). **Não** copie o `wails.exe.manifest` do instalador, que pede administrador.
- Mesmo comportamento dos scripts de hoje:
  - Portas: trio P, P+1000 e P+3000, a partir de `CD2_PORT` (padrão 8080), até 20 tentativas.
  - Contexto: `CD2_CTX` (padrão 8192).
  - Escuta só em `127.0.0.1`.
  - Chave da sessão: 32 bytes em hexadecimal, que vão para o `cd2-ctl` só pelo ambiente (`CD2_CTL_KEY`) e para a janela só pela URL.
  - URL da conversa: `http://127.0.0.1:P/?v=<VERSION>#k=<chave>`.
- Flags do motor: `-m <modelo> --server --host 127.0.0.1 --port P --jinja -c <ctx> --path <raiz>/web`.
  - Primeira tentativa com `-ngl 999`.
  - Se falhar, `--gpu disable -ngl 0`.
  - Até 240 s por tentativa.
  - No Mac, o log da tentativa com GPU é guardado em `server-gpu.log`.
- `cd2-ctl`: flags `--root <raiz> --cache <cache> --harness-port P --parent-pid <pid do lançador>`. Está no ar quando `GET http://127.0.0.1:P+3000/status` responde 401. São até 10 tentativas, uma por segundo. Se não subir, a IA segue sem banco.
- Pastas:
  - Cache: `%LOCALAPPDATA%\cd2-harness` no Windows e `~/Library/Caches/cd2-harness` no Mac.
  - Logs: `%TEMP%\cd2-harness\{server.log,server.err.log,ctl.log,ctl.err.log}` no Windows e `~/Library/Caches/cd2-harness/{server.log,server-gpu.log,ctl.log}` no Mac.
- O motor e o `cd2-ctl`:
  - No Mac, rodam de uma cópia no cache (copiada só se for diferente, com `chmod 755` e sem `com.apple.quarantine`).
  - No Windows, rodam direto do pen drive (`bin\llamafile.exe`, `bin\ctl\windows-amd64\cd2-ctl.exe`).
- Navegador, em modo `--app=<url>` com `--start-maximized`:
  - **Windows:** Edge (`%ProgramFiles(x86)%`, depois `%ProgramFiles%`) e depois Chrome (`%ProgramFiles%`, `%ProgramFiles(x86)%`, `%LOCALAPPDATA%`).
  - **Mac:** "Google Chrome", "Microsoft Edge", "Brave Browser", "Chromium" em `/Applications`, com `open -na <app> --args --app=<url> --start-maximized --window-position=0,0 --window-size=<área útil>`.
  - Sem nenhum desses, o navegador padrão.
- Encerrar tudo: no Windows, a árvore de processos; no Mac, TERM e, depois de 10 s, KILL. "Pode tirar o pen drive" só aparece depois que tudo foi encerrado.
- Uma IA por lançador no computador. Um segundo lançador aberto traz a janelinha do primeiro, que mostra "Me chamou?" se já estiver acordada.
- Visual:
  - Pixel art de `core/robo.txt` (pen, roboA, roboB, bloco, cérebro, sono, acordado).
  - Fundo `#0d1233`, amarelo `#dbff2a`, cinza `#a1a6ae`, azul `#0000ff`, fonte Manrope.
  - Piadas de carregamento iguais a `PIADAS` de `installer/frontend/dist/mascote.js`.
- Textos fixos (exatos):
  - Carregando: `carregando o modelo de IA ... {pct}%`.
  - Aviso da GPU: `Aceleração por GPU indisponível, tentando só com CPU...`.
  - Pronta: `Acordei! A conversa abriu no navegador.`.
  - Botões: `Abrir uma nova janela da conversa` e `Colocar a IA para dormir`.
  - Menu do ícone: `IA de Bolso · acordada`, `IA de Bolso · carregando {pct}%`, `Mostrar a IA de Bolso`.
  - Chamada: `Me chamou?`.
  - Dormindo: `Boa noite! Pode tirar o pen drive.`.
  - Notificação do Windows na primeira vez que a janelinha fecha: `Continuo acordada aqui perto do relógio. Me chama quando quiser.`.
  - Faixa da conversa: `A IA está dormindo. Abra o lançador do pen drive para acordá-la.`.
  - Erros: `O modelo de IA não está neste pen drive. Grave o pen drive de novo.` e `A IA não conseguiu acordar.`.
- Escolhas minhas de texto (o Gabriel pode mudar):
  - Motor ausente: `O motor da IA não está neste pen drive. Grave o pen drive de novo.`.
  - IA que morreu depois de pronta: `A IA parou de responder.`.
  - Banco: `O controle do banco não subiu. Seguindo sem banco de dados (Demonstração e Archeo continuam).`.
  - Botões de erro: `Abrir a pasta dos registros` e `Fechar`.
- Ritmos (escolhas minhas):
  - A piada troca a cada 6 s, sem repetir a anterior.
  - Depois de dormir, a janelinha fecha em 3 s.
  - A conversa confere `/health` a cada 10 s e mostra a faixa depois de 2 falhas seguidas.
- Plano B: se o lançador de uma plataforma não estiver na cópia gravada, o `INICIAR` dessa plataforma fica na raiz. Se estiver, o `INICIAR` vai para `core/`. O Linux não muda.
- Versão: `VERSION` vai para `0.14.0`, junto com todo `?v=` de `web/index.html`.
- Todo texto de interface, comentário e mensagem de commit em português brasileiro, no estilo do repositório.

## Review Focus

1. **Pen drive com caminho estranho** (`E:\`, raiz com espaço ou acento, `/Volumes/CD2 1`). O motor e o `cd2-ctl` precisam receber o caminho inteiro e certo. O teste está na Task 3 (`TestArgsCaminhoComEspacoERaizDeDrive`).
2. **Fechar a janelinha no meio da leitura do modelo.** O carregamento continua e o navegador abre do mesmo jeito no fim. O teste está na Task 6 (`TestSessaoContinuaSemJanela`).
3. **Dormir antes de ficar pronta.** Tudo é encerrado, e nenhum navegador abre depois. O teste está na Task 6 (`TestDormirDuranteCarregamento`).
4. **O motor morre sozinho depois de pronta** (pen drive arrancado, falta de memória). A janelinha mostra `A IA parou de responder.` e não deixa nenhum processo rodando. O teste está na Task 6 (`TestMotorMorreDepoisDePronta`).
5. **Gravar num computador sem os lançadores** (checkout do git ou ZIP antigo). O pen drive sai com o `INICIAR` na raiz e funciona como hoje. Os testes estão na Task 11 (`TestArrumarLancadoresSemLancador` e `lancadores.sh sem lançador`).

---

### Task 0: Spike da janela com ícone e dos avisos do sistema (descartável)

Prova os três riscos do spec antes de construir. Nada daqui entra no `main`.

**Files:**
- Create (branch descartável `spike-lancador`, nunca integrado): `spike/lancador/...`
- Modify: `docs/superpowers/specs/2026-10-17-lancador-pendrive-design.md` (seção nova "Resultado do spike")

- [ ] **Step 1: Montar o protótipo A, Wails v2.10.1 + `fyne.io/systray`.** Janela de 320×300 com `HideWindowOnClose: true` e `SingleInstanceLock` (o `OnSecondInstanceLaunch` mostra a janela). Ícone de bandeja com os itens "Mostrar" e "Sair". No Mac, a bandeja sobe por `systray.RunWithExternalLoop` dentro do loop do Wails. No Windows, `systray.Run` roda numa goroutine com `runtime.LockOSThread`.
- [ ] **Step 2: Conferir no Mac (Apple Silicon) e pedir ao Gabriel que confira no Windows**, com o binário do CI como artefato. Os pontos são:
  - a janela abre;
  - fechar esconde a janela e o ícone continua;
  - "Mostrar" traz a janela de volta;
  - um segundo duplo clique traz a janela;
  - "Sair" fecha tudo sem travar.
  Tudo isso passa 10 vezes seguidas.
- [ ] **Step 3: Se o A falhar em qualquer ponto, montar o protótipo B com Wails v3** (janela + `application.SystemTray` nativo) e repetir o Step 2.
- [ ] **Step 4: Gravar um pen drive de teste e copiar o `.app` e o `.exe` assinados ad hoc para a raiz.** Use os dois caminhos: o instalador e o `GRAVAR-PENDRIVE-Mac.command` ou `-Windows.bat`. Anote se aparece o aviso do Gatekeeper ou do SmartScreen nos dois dois cliques. No Windows, anote também se a janela abre sem internet num Windows 10. Se o WebView2 faltar, anote a mensagem.
- [ ] **Step 5: Registrar o resultado no spec, em "Resultado do spike".** Registre a biblioteca escolhida, o que apareceu em cada aviso e, para cada plataforma, se o lançador vai ou se fica o plano B (a decisão do Gabriel). Commit só desse texto:

```bash
git add docs/superpowers/specs/2026-10-17-lancador-pendrive-design.md
git commit -m "Lançador: resultado do spike de janela, bandeja e avisos do sistema"
```

**Checkpoint com o Gabriel:** só siga para as Tasks 7 a 9 com a biblioteca registrada. As Tasks 1 a 6 não dependem dela e podem andar em paralelo.

---

### Task 1: Módulo `launcher/`, raiz do pen drive, portas e chave

**Files:**
- Create: `launcher/go.mod` (module `github.com/cd2tec/cd2-ia-de-bolso/launcher`, `go 1.23`)
- Create: `launcher/internal/raiz/raiz.go`, `launcher/internal/raiz/raiz_test.go`
- Create: `launcher/internal/portas/portas.go`, `launcher/internal/portas/portas_test.go`
- Create: `launcher/internal/chave/chave.go`, `launcher/internal/chave/chave_test.go`
- Modify: `.github/workflows/ci.yml` (job `launcher`, igual ao job `installer`: gofmt, `go vet` em linux/windows/darwin e `go test ./internal/...`)

**Interfaces:**
- Produces:
  - `raiz.DoExecutavel(exe string, goos string) (string, error)`. No Windows, devolve a pasta do `.exe`. No Mac, devolve a pasta que contém o `.app` (sobe de `X/IA de Bolso.app/Contents/MacOS/IA de Bolso` para `X`). É erro se o `.app` não estiver no caminho.
  - `portas.Achar(inicio int, livre func(int) bool) (int, error)`: devolve o primeiro P com P, P+1000 e P+3000 livres, tentando de `inicio` a `inicio+19`. É erro se nenhum servir.
  - `portas.Livre(p int) bool`: tenta escutar em `127.0.0.1:p`.
  - `chave.Nova() (string, error)`: 64 caracteres hexadecimais em minúsculas, de `crypto/rand`.

- [ ] **Step 1: Escrever os testes que falham**

```go
func TestRaizMac(t *testing.T) {
	r, err := DoExecutavel("/Volumes/CD2 1/IA de Bolso.app/Contents/MacOS/IA de Bolso", "darwin")
	// r == "/Volumes/CD2 1", err == nil
}
func TestRaizMacForaDoApp(t *testing.T) { _, err := DoExecutavel("/tmp/IA de Bolso", "darwin") /* err != nil */ }
func TestRaizWindows(t *testing.T) { r, _ := DoExecutavel(`E:\IA de Bolso.exe`, "windows") /* r == `E:\` */ }
func TestAcharPulaTrioOcupado(t *testing.T) {
	ocupadas := map[int]bool{8080: true, 9081: true}
	p, err := Achar(8080, func(p int) bool { return !ocupadas[p] })
	// p == 8082, err == nil   (8080 ocupada; 8081 cai porque 9081 está ocupada)
}
func TestAcharDesisteDepoisDe20(t *testing.T) { _, err := Achar(8080, func(int) bool { return false }) /* err != nil */ }
func TestChaveNova(t *testing.T) { /* len == 64, casa ^[0-9a-f]{64}$, duas chamadas diferentes */ }
```

- [ ] **Step 2: Rodar e ver falhar.** Rode `cd launcher && go test ./internal/...`. O esperado é FAIL (`undefined: DoExecutavel`...).
- [ ] **Step 3: Implementar as três funções e o `Livre`.** No `DoExecutavel`, use `path/filepath` com o separador da plataforma passada (no Windows, `strings` sobre `\`), porque os testes rodam no Linux.
- [ ] **Step 4: Rodar e ver passar.** Rode `cd launcher && go test ./internal/... && test -z "$(gofmt -l .)"`. O esperado é PASS.
- [ ] **Step 5: Commit**

```bash
git add launcher/go.mod launcher/internal/raiz launcher/internal/portas launcher/internal/chave .github/workflows/ci.yml
git commit -m "Lançador: módulo novo com raiz do pen drive, trio de portas e chave da sessão"
```

---

### Task 2: Leitura do modelo com progresso e cópia para o cache

**Files:**
- Create: `launcher/internal/modelo/modelo.go`, `launcher/internal/modelo/modelo_test.go`
- Create: `launcher/internal/cache/cache.go`, `cache_darwin.go`, `cache_outros.go`, `launcher/internal/cache/cache_test.go`

**Interfaces:**
- Produces:
  - `modelo.Achar(raiz string) (string, error)`: devolve o primeiro `models/*.gguf` em ordem alfabética, ou `modelo.ErrSemModelo`.
  - `modelo.Ler(ctx context.Context, caminho string, bloco int64, prog func(feitos, total int64)) error`: lê o arquivo inteiro em blocos de `bloco` bytes (o padrão é 32 MiB, `modelo.Bloco`) e chama `prog` depois de cada bloco e uma última vez com `feitos == total`. Respeita o `ctx`.
  - `cache.Pasta(goos string, env func(string) string, home string) string`: a pasta de cache da plataforma.
  - `cache.PastaLogs(goos string, env func(string) string, home string) string`: `%TEMP%\cd2-harness` no Windows; no Mac, a mesma pasta do cache.
  - `cache.Preparar(origem, destino string) (copiou bool, err error)`: copia só se o destino não existir ou for diferente byte a byte. Copia para `destino+".tmp"`, faz `chmod 755` e renomeia. No darwin, tira `com.apple.quarantine` (o `xattr`, ignorando erro).

- [ ] **Step 1: Escrever os testes que falham**

```go
func TestLerProgressoAteOFim(t *testing.T) {
	// arquivo de 100 bytes, bloco 32 -> prog recebe (32,100) (64,100) (96,100) (100,100); err == nil
}
func TestLerCanceladoPara(t *testing.T) { /* ctx cancelado antes -> err == context.Canceled, nenhum prog com feitos==total */ }
func TestAcharSemModelo(t *testing.T) { /* models/ vazio -> errors.Is(err, ErrSemModelo) */ }
func TestPrepararCopiaSoQuandoMuda(t *testing.T) {
	// 1a chamada: copiou == true, modo 0755 (fora do Windows); 2a igual: copiou == false;
	// muda 1 byte na origem: copiou == true e o conteúdo bate
}
func TestPastaPorSistema(t *testing.T) {
	// Pasta("windows", env{LOCALAPPDATA:`C:\U\AppData\Local`}, "") == `C:\U\AppData\Local\cd2-harness`
	// Pasta("darwin", nil, "/Users/g") == "/Users/g/Library/Caches/cd2-harness"
	// PastaLogs("windows", env{TEMP:`C:\T`}, "") == `C:\T\cd2-harness`
}
```

- [ ] **Step 2: Rodar e ver falhar.** Rode `cd launcher && go test ./internal/modelo ./internal/cache`. O esperado é FAIL.
- [ ] **Step 3: Implementar.** No `Preparar`, compare primeiro o tamanho e só depois o conteúdo (`bytes.Equal` em blocos), como o `cmp -s` do script.
- [ ] **Step 4: Rodar e ver passar.** Mesmo comando. O esperado é PASS.
- [ ] **Step 5: Commit**

```bash
git add launcher/internal/modelo launcher/internal/cache
git commit -m "Lançador: leitura do modelo com progresso real e cópia do motor para o cache"
```

---

### Task 3: Motor (llamafile), com GPU, queda para CPU e encerramento

**Files:**
- Create: `launcher/internal/motor/motor.go`, `proc_windows.go`, `proc_unix.go`, `launcher/internal/motor/motor_test.go`

**Interfaces:**
- Consumes: nada das outras tasks (recebe caminhos prontos).
- Produces:
  - `type Config struct { Exe, Modelo, Web, LogDir string; Porta int; Ctx string; GOOS string }`
  - `motor.Args(c Config, gpu bool) []string`: as flags exatas das Global Constraints (`-ngl 999`, ou `--gpu disable -ngl 0`).
  - `type Processo struct{ ... }` com `Morreu() <-chan struct{}` e `Parar() error`. O `Parar` é idempotente: no Windows, `taskkill /PID <pid> /T /F`; fora dele, TERM, espera até 10 s e depois KILL.
  - `motor.Iniciar(c Config, gpu bool) (*Processo, error)`: escreve stdout e stderr em `server.log` (no Windows, `server.err.log` à parte) e zera o log antes. No Windows, usa `SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}`.
  - `motor.Esperar(ctx context.Context, url string, p *Processo, prazo time.Duration, get func(string) (int, error)) error`: confere `GET url+"/health"` a cada 1 s. Volta `nil` com 200, `motor.ErrMorreu` se o processo sair e `motor.ErrPrazo` se o prazo estourar.
  - `motor.Subir(ctx, c Config, get, aviso func(string)) (*Processo, error)`: tenta com GPU. Se falhar, chama `aviso("Aceleração por GPU indisponível, tentando só com CPU...")`, para o processo, guarda `server.log` como `server-gpu.log` e tenta com CPU. Se a CPU também falhar, devolve `motor.ErrNaoSubiu`. O prazo é de 240 s por tentativa (`motor.Prazo`).

- [ ] **Step 1: Escrever os testes que falham.** Use o padrão `TestHelperProcess`: o próprio binário de teste faz o papel do llamafile, conforme a variável `MOTOR_FALSO`, que pode ser `ok`, `gpu-falha` ou `morre`.

```go
func TestArgsCaminhoComEspacoERaizDeDrive(t *testing.T) {
	a := Args(Config{Modelo: `E:\models\m.gguf`, Web: `E:\web`, Porta: 8081, Ctx: "8192"}, true)
	// a == []string{"-m", `E:\models\m.gguf`, "--server", "--host", "127.0.0.1", "--port", "8081",
	//               "--jinja", "-c", "8192", "--path", `E:\web`, "-ngl", "999"}
	// (exec.Command cuida das aspas: nada de aspas manuais nos argumentos)
}
func TestArgsCPU(t *testing.T) { /* termina em "--gpu", "disable", "-ngl", "0" */ }
func TestSubirCaiParaCPU(t *testing.T) {
	// MOTOR_FALSO=gpu-falha: a 1a tentativa morre, a 2a fica no ar; aviso chamado 1 vez com o texto exato;
	// server-gpu.log existe; get falso devolve 200 só depois da 2a tentativa
}
func TestSubirDesiste(t *testing.T) { /* MOTOR_FALSO=morre -> errors.Is(err, ErrNaoSubiu); nenhum processo vivo */ }
func TestPararIdempotente(t *testing.T) { /* Parar() duas vezes -> nil; Morreu() fechado */ }
```

- [ ] **Step 2: Rodar e ver falhar.** Rode `cd launcher && go test ./internal/motor`. O esperado é FAIL.
- [ ] **Step 3: Implementar.** Nos testes, use um prazo curto, com `motor.Prazo` como variável do pacote que o teste troca. `Esperar` também volta assim que `Morreu()` fecha, sem esperar o próximo segundo.
- [ ] **Step 4: Rodar e ver passar**, também com `GOOS=windows go vet ./internal/motor`. O esperado é PASS e vet sem erro.
- [ ] **Step 5: Commit**

```bash
git add launcher/internal/motor
git commit -m "Lançador: motor com tentativa na GPU, queda para CPU e encerramento da árvore"
```

---

### Task 4: Controle do banco (cd2-ctl)

**Files:**
- Create: `launcher/internal/ctl/ctl.go`, `launcher/internal/ctl/ctl_test.go`

**Interfaces:**
- Consumes: `motor.Processo` (Task 3), reaproveitado para iniciar e parar processos. Se for mais limpo, extraia `Iniciar` e `Parar` para `launcher/internal/proc` e faça o motor usar esse pacote.
- Produces:
  - `ctl.Caminho(raiz, goos, goarch string) string`: `bin/ctl/<windows-amd64|darwin-arm64|darwin-amd64>/cd2-ctl[.exe]`. No darwin, `arm64` vira `darwin-arm64` e o resto vira `darwin-amd64`.
  - `ctl.Args(raiz, cache string, portaHarness, pidPai int) []string`: `--root`, `--cache`, `--harness-port`, `--parent-pid`.
  - `ctl.Subir(ctx, exe string, args []string, chave string, portaCtl int, logDir string, get func(string) (int, error)) (*motor.Processo, error)`: põe `CD2_CTL_KEY=<chave>` **só** no `Env` do processo filho, nunca no do lançador. Confere `GET http://127.0.0.1:<portaCtl>/status` 10 vezes, uma por segundo, e 401 significa no ar. Se falhar, para o processo e devolve `ctl.ErrNaoSubiu`, com as últimas 5 linhas do log no erro.

- [ ] **Step 1: Escrever os testes que falham**

```go
func TestCaminhoPorPlataforma(t *testing.T) {
	// ("E:\\", "windows","amd64") -> `E:\bin\ctl\windows-amd64\cd2-ctl.exe`; ("/V/CD2","darwin","arm64") -> "/V/CD2/bin/ctl/darwin-arm64/cd2-ctl"
}
func TestSubirChaveSoNoFilho(t *testing.T) {
	// processo falso grava os.Getenv("CD2_CTL_KEY") num arquivo; == chave; os.Getenv("CD2_CTL_KEY") do teste == ""
}
func TestSubir401EstaNoAr(t *testing.T) { /* get devolve 401 na 2a chamada -> err == nil */ }
func TestSubirFalhaTrazFimDoLog(t *testing.T) { /* processo falso escreve "porta em uso" e sai -> errors.Is(err, ErrNaoSubiu) e err.Error() contém "porta em uso" */ }
```

- [ ] **Step 2: Rodar e ver falhar.** Rode `cd launcher && go test ./internal/ctl`. O esperado é FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Rodar e ver passar.** O esperado é PASS.
- [ ] **Step 5: Commit**

```bash
git add launcher/internal/ctl
git commit -m "Lançador: sobe o cd2-ctl com a chave só no ambiente dele"
```

---

### Task 5: Navegador em modo janela

**Files:**
- Create: `launcher/internal/navegador/navegador.go`, `tela_darwin.go`, `tela_outros.go`, `launcher/internal/navegador/navegador_test.go`

**Interfaces:**
- Produces:
  - `navegador.URL(porta int, versao, chave string) string`: `http://127.0.0.1:<porta>/?v=<versao>#k=<chave>`.
  - `navegador.Comando(goos string, url string, existe func(string) bool, env func(string) string, tela string) (nome string, args []string)`: monta o comando exato das Global Constraints. `tela` é `"L,A"` ou vazio. Sem navegador conhecido, devolve o abridor padrão: `rundll32 url.dll,FileProtocolHandler <url>` no Windows e `open <url>` no Mac.
  - `navegador.AreaUtil() string`: no Mac, o mesmo `osascript -l JavaScript` de `core/start-unix.sh` (`tela_mac`); fora do Mac, vazio.
  - `navegador.Abrir(url string) error`: junta tudo e dispara sem esperar o processo.

- [ ] **Step 1: Escrever os testes que falham**

```go
func TestURL(t *testing.T) { /* URL(8080,"0.14.0","ab") == "http://127.0.0.1:8080/?v=0.14.0#k=ab" */ }
func TestWindowsPrefereEdge(t *testing.T) {
	// existe só para `C:\PF86\Microsoft\Edge\Application\msedge.exe` e o chrome -> nome == edge, args == {"--app=<url>","--start-maximized"}
}
func TestWindowsSoChromeLocal(t *testing.T) { /* só %LOCALAPPDATA%\Google\Chrome\Application\chrome.exe -> usa ele */ }
func TestMacChromeComTamanho(t *testing.T) {
	// existe("/Applications/Google Chrome.app") -> nome "open", args == {"-na","Google Chrome","--args","--app=<url>",
	//   "--start-maximized","--window-position=0,0","--window-size=1440,875"} com tela "1440,875"
}
func TestMacSemTamanho(t *testing.T) { /* tela "" -> sem --window-position nem --window-size */ }
func TestSemNavegadorConhecido(t *testing.T) { /* darwin -> ("open", {url}); windows -> ("rundll32", {"url.dll,FileProtocolHandler", url}) */ }
```

- [ ] **Step 2: Rodar e ver falhar.** Rode `cd launcher && go test ./internal/navegador`. O esperado é FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Rodar e ver passar.** O esperado é PASS.
- [ ] **Step 5: Commit**

```bash
git add launcher/internal/navegador
git commit -m "Lançador: abre a conversa no Edge ou no Chrome em modo janela"
```

---

### Task 6: Sessão (o maestro do carregamento ao sono)

**Files:**
- Create: `launcher/internal/sessao/sessao.go`, `launcher/internal/sessao/estado.go`, `launcher/internal/sessao/sessao_test.go`

**Interfaces:**
- Consumes: `raiz`, `portas`, `chave` (Task 1); `modelo` e `cache` (Task 2); `motor` (Task 3); `ctl` (Task 4); `navegador` (Task 5).
- Produces:
  - `type Fase string` com as constantes `Lendo`, `Subindo`, `Pronta`, `Dormindo`, `Dormiu`, `Erro`.
  - `type Estado struct { Fase Fase; Pct int; Aviso string; Erro string; URL string }`. `URL` só vem preenchida em `Pronta`. `Pct` vai de 0 a 100: a leitura do modelo ocupa de 0 a 90, e a espera pelo servidor de 90 a 99. `Erro` recebe um dos textos fixos.
  - `type Deps struct` com uma função para cada efeito (`AcharModelo`, `Ler`, `PrepararCache`, `SubirMotor`, `SubirCtl`, `AbrirNavegador`, `Agora`...), para os testes usarem falsos. `sessao.DepsReais(raiz string) Deps` liga os pacotes de verdade.
  - `sessao.Nova(d Deps, versao string, aoMudar func(Estado)) *Sessao`
  - `(*Sessao).Rodar(ctx context.Context)`: a sequência dos scripts (achar o modelo, preparar o cache no Mac, achar as portas, criar a chave, ler o modelo, subir o motor, subir o `cd2-ctl`, abrir o navegador, ficar `Pronta`) e depois vigiar o motor.
  - `(*Sessao).AbrirConversa() error`: abre outra janela com a mesma URL. É erro fora de `Pronta`.
  - `(*Sessao).Dormir()`: idempotente. Cancela o carregamento, para o `cd2-ctl` e o motor, emite `Dormindo` e, depois que tudo parou, `Dormiu`.
  - `(*Sessao).Atual() Estado`

- [ ] **Step 1: Escrever os testes que falham**, todos com `Deps` falsos e sem processo de verdade.

```go
func TestSessaoFeliz(t *testing.T) {
	// fases emitidas em ordem: Lendo(pct crescente até 90) -> Subindo -> Pronta; AbrirNavegador chamado 1x com
	// "http://127.0.0.1:8080/?v=0.14.0#k=<chave>"; Atual().URL igual
}
func TestSessaoAvisoGPU(t *testing.T) { /* SubirMotor chama aviso -> algum Estado com Aviso == texto exato da GPU */ }
func TestSessaoSemModelo(t *testing.T) { /* AcharModelo -> ErrSemModelo -> último Estado {Fase: Erro, Erro: "O modelo de IA não está neste pen drive. Grave o pen drive de novo."}; SubirMotor nunca chamado */ }
func TestSessaoMotorNaoSobe(t *testing.T) { /* Erro == "A IA não conseguiu acordar." */ }
func TestSessaoCtlFalhaSegue(t *testing.T) { /* SubirCtl erro -> chega a Pronta com Aviso == texto do banco */ }
func TestSessaoContinuaSemJanela(t *testing.T) {
	// aoMudar ignora tudo (janela fechada) e Rodar chega a Pronta e abre o navegador do mesmo jeito
}
func TestDormirDuranteCarregamento(t *testing.T) {
	// Dormir() enquanto Ler está bloqueado -> Ler recebe ctx cancelado; AbrirNavegador nunca chamado;
	// motor e ctl parados (se já tinham subido); últimas fases Dormindo -> Dormiu
}
func TestDormirDepoisDePronta(t *testing.T) { /* Parar do ctl antes do Parar do motor; Dormiu só depois dos dois; 2a chamada não faz nada */ }
func TestMotorMorreDepoisDePronta(t *testing.T) {
	// fecha Morreu() do motor falso -> Estado {Fase: Erro, Erro: "A IA parou de responder."}; ctl parado
}
func TestAbrirConversaSoPronta(t *testing.T) { /* em Lendo -> erro; em Pronta -> AbrirNavegador com a mesma URL */ }
```

- [ ] **Step 2: Rodar e ver falhar.** Rode `cd launcher && go test ./internal/sessao`. O esperado é FAIL.
- [ ] **Step 3: Implementar.** Um mutex protege o estado. O `aoMudar` é chamado fora do lock. `Dormir` usa `sync.Once`.
- [ ] **Step 4: Rodar e ver passar, também com o detector de corridas.** Rode `cd launcher && go test -race ./internal/...`. O esperado é PASS.
- [ ] **Step 5: Commit**

```bash
git add launcher/internal/sessao
git commit -m "Lançador: sessão que carrega, abre a conversa, vigia o motor e põe a IA para dormir"
```

---

### Task 7: Ícone do cérebro em pixel art

**Files:**
- Create: `launcher/internal/icone/icone.go`, `launcher/internal/icone/icone_test.go`

**Interfaces:**
- Produces:
  - `icone.Cerebro(estado string, escala int) []byte`: um PNG. `estado` pode ser `"carregando"` (cérebro meio cheio, rosto `sono`) ou `"acordada"` (cheio, rosto `acordado`). Os pixels são os de `[cerebro]`, `[sono]` e `[acordado]` de `core/robo.txt`, com `p` e `g` em `#a1a6ae`, `i` cheio em `#dbff2a`, vazio em `#1b2050`, `k` em `#0d1233`, `w` em `#ffffff` e `.` transparente.
  - `icone.ICO(png []byte) []byte`: o PNG embrulhado em `.ico`, para a bandeja do Windows.

- [ ] **Step 1: Escrever os testes que falham**

```go
func TestCerebroTamanhoETransparencia(t *testing.T) {
	// Cerebro("acordada", 2) decodifica como PNG 20x16; pixel (0,0) com alfa 0; pixel do contorno (4,0) == #a1a6ae
}
func TestCerebroCarregandoMeioVazio(t *testing.T) { /* linha 1 (topo de dentro) tem #1b2050; linha 6 tem #dbff2a */ }
func TestPixelsIguaisAoRoboTxt(t *testing.T) {
	// lê ../../../core/robo.txt, extrai [cerebro], [sono], [acordado] e compara com as grades do pacote
}
func TestICO(t *testing.T) { /* começa com 00 00 01 00, 1 imagem, e o PNG inteiro vem embutido */ }
```

- [ ] **Step 2: Rodar e ver falhar.** Rode `cd launcher && go test ./internal/icone`. O esperado é FAIL.
- [ ] **Step 3: Implementar com `image/png`.**
- [ ] **Step 4: Rodar e ver passar.** O esperado é PASS.
- [ ] **Step 5: Commit**

```bash
git add launcher/internal/icone
git commit -m "Lançador: ícone do cérebro em pixel art, igual ao robo.txt"
```

---

### Task 8: Telas da janelinha (HTML, CSS e JS)

**Files:**
- Create: `launcher/frontend/dist/index.html`, `estilo.css`, `cena.js`, `piadas.js`, `tela.js`, `fonts/manrope-*.woff2` (cópia de `installer/frontend/dist/fonts/`)
- Create: `launcher/frontend/dist/mascote.js` (cópia byte a byte de `installer/frontend/dist/mascote.js`)
- Test: `tests/lancador-cena.test.js`, `tests/lancador-tela.test.js`

**Interfaces:**
- Consumes: o `Estado` da Task 6, que chega como evento `estado` com JSON `{fase, pct, aviso, erro}`, e o evento `chamada`, sem dados. Os métodos ligados (Task 9) são `AbrirConversa()`, `Dormir()`, `AbrirRegistros()` e `Fechar()`.
- Produces:
  - `cena.js` exporta `CD2Cena = { QUADROS, svgCarregando(pct, passo), svgCerebro(rosto, cheio) }`. `QUADROS` traz os pixels de `core/robo.txt`. `rosto` pode ser `'sono'`, `'acordado'` ou `'espia'`.
  - `piadas.js` exporta `CD2Piadas = { CHAMADA, DESPEDIDA, proxima(lista, ultima, sorteio) }`. `CHAMADA` tem pelo menos 6 frases de quem foi chamada, incluindo `Tava aqui no cantinho do relógio, contando os segundos. Literalmente.`. `DESPEDIDA` tem pelo menos 4, incluindo `Vou sonhar com planilhas. Até a próxima.`. As de carregamento vêm de `CD2Mascote.PIADAS`.
  - `tela.js` exporta `CD2Tela = { render(estado, modo) }`, que devolve `{titulo, linha, piadaLista, botoes}`. `modo` pode ser `'normal'` ou `'chamada'`. Também liga o DOM aos eventos do Wails.

- [ ] **Step 1: Escrever os testes que falham**

```js
test('mascote.js do lançador é cópia do instalador', () => { /* fs.readFileSync dos dois === */ });
test('cena usa os pixels de core/robo.txt', () => { /* parse de [pen] [roboA] [roboB] [bloco] [cerebro] [sono] [acordado] deepEqual CD2Cena.QUADROS */ });
test('render Lendo', () => {
  // render({fase:'Lendo',pct:62},'normal') -> linha === 'carregando o modelo de IA ... 62%', piadaLista === PIADAS, botoes == []
});
test('render Pronta normal', () => {
  // titulo 'Acordei! A conversa abriu no navegador.'; botoes ['Abrir uma nova janela da conversa','Colocar a IA para dormir']
});
test('render Pronta chamada', () => { /* titulo 'Me chamou?'; piadaLista === CHAMADA; mesmos botões */ });
test('render Lendo chamada mostra o carregamento', () => { /* modo chamada em Lendo -> igual ao normal */ });
test('render Dormiu', () => { /* titulo 'Boa noite! Pode tirar o pen drive.'; piadaLista === DESPEDIDA; botoes [] */ });
test('render Erro', () => { /* titulo === estado.erro; botoes ['Abrir a pasta dos registros','Fechar'] */ });
test('aviso aparece como linha', () => { /* {fase:'Subindo',aviso:'Aceleração...'} -> linha === aviso */ });
test('proxima não repete', () => { /* com sorteio que devolve sempre o mesmo índice, troca */ });
```

- [ ] **Step 2: Rodar e ver falhar.** Rode `node --test tests/lancador-cena.test.js tests/lancador-tela.test.js`. O esperado é FAIL.
- [ ] **Step 3: Implementar.** Siga o rascunho aprovado: a cena do pen drive, do robô andando e do cérebro enchendo; a barra amarela; a linha em fonte monoespaçada; e a piada com `> ` trocando a cada 6 s. Use o mesmo formato de módulo de `mascote.js` (UMD pequeno, `module.exports` ou `window`). Nada de fontes ou scripts da internet.
- [ ] **Step 4: Rodar e ver passar.** Rode `node --test`. O esperado é PASS em todos, inclusive nos antigos.
- [ ] **Step 5: Commit**

```bash
git add launcher/frontend tests/lancador-cena.test.js tests/lancador-tela.test.js
git commit -m "Lançador: telas da janelinha com a cena do robô e as piadas"
```

---

### Task 9: A casca, com janela, ícone, segunda cópia e build

**Files:**
- Create: `launcher/main.go`, `launcher/janela.go`, `launcher/bandeja.go`, `launcher/bandeja_windows.go`, `launcher/bandeja_darwin.go` (separe por plataforma só se a biblioteca do spike exigir)
- Create: `launcher/wails.json` (`"name": "IA de Bolso"`, `"outputfilename": "IA de Bolso"`, frontend em `frontend/dist`, sem passo de npm)
- Create: `launcher/build/darwin/Info.plist` (`CFBundleName` "IA de Bolso", `LSUIElement` `true`, ou seja, sem ícone no Dock), `launcher/build/windows/wails.exe.manifest` (`asInvoker`), `launcher/build/appicon.png` (o cérebro acordado de `icone.Cerebro`, com escala de 1024 px)
- Modify: `launcher/go.mod` (dependências da biblioteca escolhida)

**Interfaces:**
- Consumes: `sessao` (Task 6), `icone` (Task 7) e as telas (Task 8).
- Produces: métodos ligados `AbrirConversa() error`, `Dormir()`, `AbrirRegistros() error` (abre `cache.PastaLogs` no Explorer ou no Finder) e `Fechar()`. Eventos para a tela: `estado` e `chamada`.

Comportamento que esta task liga, tudo do spec:
- Fechar a janela só esconde: `HideWindowOnClose: true`. No Windows, a primeira vez manda a notificação de texto exato.
- Na bandeja:
  - o tooltip e o cabeçalho do menu mostram `IA de Bolso · carregando {pct}%` ou `IA de Bolso · acordada`;
  - os itens são `Mostrar a IA de Bolso`, `Abrir uma nova janela da conversa` (desligado fora de `Pronta`) e `Colocar a IA para dormir`;
  - o ícone troca de `carregando` para `acordada` em `Pronta`.
- No Windows, clique simples mostra a janela e o botão direito abre o menu. No Mac, o clique abre o menu.
- "Mostrar", o clique simples no Windows e a segunda cópia (`SingleInstanceLock`, id `br.com.cd2.iadebolso.lancador`) mostram a janela e emitem `chamada`.
- Em `Dormiu`, a janela espera 3 s e o app sai. A bandeja some no `Quit`.
- Sair do app por qualquer caminho (fechar pelo sistema, logout) chama `Dormir()` antes, no `OnBeforeClose` e no `OnShutdown`.

- [ ] **Step 1: Escrever o teste do que dá para testar sem janela.** O `launcher/janela_test.go` cobre `rotuloBandeja(Estado) string` e `itensBandeja(Estado) []item{Texto string; Ligado bool}`, que são funções puras de `bandeja.go`:

```go
func TestRotuloBandeja(t *testing.T) { /* {Lendo,62} -> "IA de Bolso · carregando 62%"; {Pronta} -> "IA de Bolso · acordada" */ }
func TestItensBandeja(t *testing.T) { /* em Lendo, "Abrir uma nova janela da conversa" desligado; em Pronta, ligado; textos exatos */ }
```

- [ ] **Step 2: Rodar e ver falhar.** Rode `cd launcher && go test .`. O esperado é FAIL.
- [ ] **Step 3: Implementar a casca com a biblioteca do spike.** `main.go` acha a raiz com `raiz.DoExecutavel(os.Executable())` e cria a sessão com `sessao.DepsReais(raiz)`. A `versao` vem por `-ldflags "-X main.versao=..."`, e, se faltar, é lida de `<raiz>/VERSION`.
- [ ] **Step 4: Rodar os testes e compilar no Mac.** Rode `cd launcher && go test ./... && wails build -clean -trimpath -platform darwin/universal && codesign --force --deep -s - "build/bin/IA de Bolso.app"`. O esperado é PASS e o `.app` gerado.
- [ ] **Step 5: Conferir na mão no Mac, com um pen drive gravado pelo `GRAVAR-PENDRIVE-Mac.command` e o `.app` copiado para a raiz.** Os pontos são:
  - dois cliques e a cena anda;
  - a conversa abre no Chrome;
  - fechar a janela no meio da leitura deixa o ícone na barra de menus, com a porcentagem;
  - a conversa abre sozinha no fim;
  - "Mostrar" mostra "Me chamou?";
  - "Abrir uma nova janela" abre outra janela;
  - dois cliques no `.app` de novo trazem a janela, sem segunda IA (`pgrep -f llamafile` mostra 1);
  - "Colocar a IA para dormir" mostra "Boa noite!" e, depois, `pgrep -f "llamafile|cd2-ctl|toolbox"` não mostra nada.
- [ ] **Step 6: Commit**

```bash
git add launcher
git commit -m "Lançador: janela, ícone na barra de menus e perto do relógio, e uma IA por vez"
```

**Checkpoint com o Gabriel:** ele repete o Step 5 no Windows com o `.exe` do artefato do CI (Task 12). Ali, o ícone fica perto do relógio, a notificação aparece na primeira vez que a janela fecha e, no fim, o Gerenciador de Tarefas não mostra `llamafile.exe` nem `cd2-ctl.exe`.

---

### Task 10: Faixa "IA dormindo" na conversa

**Files:**
- Create: `web/sono.js`
- Modify: `web/index.html` (o `<script src="sono.js?v=...">` junto dos outros, a faixa no topo e o início da vigia depois de `pingModel()` dar certo)
- Test: `tests/sono.test.js`

**Interfaces:**
- Produces: `CD2Sono.vigiar({ fetch, intervalo = 10000, falhas = 2, aoDormir, aoAcordar, timers })`, que devolve `{ parar() }`. A cada `intervalo`, faz `fetch('/health', {cache:'no-store'})`. Depois de `falhas` erros seguidos, chama `aoDormir()` uma vez. Se o `/health` voltar, chama `aoAcordar()` uma vez. `timers` (`setTimeout` e `clearTimeout`) é injetável para o teste.

- [ ] **Step 1: Escrever os testes que falham**

```js
test('uma falha não acusa sono', ...);           // 1 erro -> aoDormir não chamado
test('duas falhas seguidas acusam sono uma vez', ...); // 3 erros -> aoDormir chamado 1x
test('volta a responder e acorda', ...);         // erro, erro, ok -> aoAcordar 1x
test('parar para de vigiar', ...);
```

- [ ] **Step 2: Rodar e ver falhar.** Rode `node --test tests/sono.test.js`. O esperado é FAIL.
- [ ] **Step 3: Implementar `web/sono.js` e ligar no `web/index.html`.** A faixa usa as cores de atenção que já existem no app e o texto exato `A IA está dormindo. Abra o lançador do pen drive para acordá-la.`. Ela some no `aoAcordar`.
- [ ] **Step 4: Rodar e ver passar.** Rode `node --test`. O esperado é PASS em todos.
- [ ] **Step 5: Conferir no navegador.** Rode `sh iniciar-cd2-linux.sh` ou o `.command`, abra a conversa e mate o llamafile (`pkill -f llamafile`). Em até 20 s a faixa aparece. Tire um screenshot.
- [ ] **Step 6: Commit**

```bash
git add web/sono.js web/index.html tests/sono.test.js
git commit -m "Conversa: avisa quando a IA foi dormir"
```

---

### Task 11: Gravação põe o lançador na raiz, com plano B

**Files:**
- Modify: `INICIAR-CD2-Windows.bat` (acha o `start-windows.ps1` em `%~dp0core\` ou em `%~dp0`, o que existir)
- Modify: `INICIAR-CD2-Mac.command` (se existir `core/start-unix.sh` ao lado, roda dali; senão, sobe uma pasta)
- Create: `core/lancadores.sh` (função `arrumar_lancadores <stage>`, que o `make-usb-unix.sh` usa)
- Modify: `core/make-usb-unix.sh:96-100` (copia `IA de Bolso.exe` e `IA de Bolso.app` se existirem, com `cp -R`/`cp -RX`, e chama `arrumar_lancadores "$STAGE"`)
- Modify: `core/make-usb-windows.ps1:78-82` (mesma regra) e `Find-Cd2Starter` (prefere `IA de Bolso.exe` ao `.bat`)
- Modify: `core/prepare-unix.sh:139` e `core/prepare-windows.ps1` (a linha equivalente do `CHECKSUMS`): incluem `IA de Bolso.exe` e `IA de Bolso.app/Contents/MacOS/IA de Bolso` quando existirem
- Modify: `installer/internal/preparo/preparo.go` (`itensDoZip` aceita `IA de Bolso.exe` e o prefixo `IA de Bolso.app/`; arquivos em `Contents/MacOS/` ficam com 0755; nova `arrumarLancadores(stage string) error`, chamada depois de `extrair`)
- Modify: `installer/internal/preparo/checksums.go` (`GerarChecksums` inclui os dois executáveis do lançador quando existirem)
- Test: `installer/internal/preparo/preparo_test.go`, `tests/lancadores.test.js`

**Interfaces:**
- Produces: a regra única, igual nos três lugares. Para cada plataforma (Windows: `IA de Bolso.exe` ↔ `INICIAR-CD2-Windows.bat`; Mac: `IA de Bolso.app` ↔ `INICIAR-CD2-Mac.command`):
  - se o lançador existe na cópia, o `INICIAR` vai da raiz para `core/`;
  - se não existe, o `INICIAR` fica na raiz.
  - O `iniciar-cd2-linux.sh` nunca se move.

- [ ] **Step 1: Escrever os testes que falham**

```go
func TestArrumarLancadoresComLancador(t *testing.T) {
	// stage com "IA de Bolso.exe", "IA de Bolso.app/Contents/MacOS/IA de Bolso", os dois INICIAR na raiz ->
	// INICIAR-CD2-Windows.bat e INICIAR-CD2-Mac.command só em core/; iniciar-cd2-linux.sh na raiz
}
func TestArrumarLancadoresSemLancador(t *testing.T) { /* nada muda */ }
func TestArrumarLancadoresSoMac(t *testing.T) { /* só o .app -> .command em core/, .bat na raiz */ }
func TestExtrairLevaOLancador(t *testing.T) { /* zip com o .exe e o .app -> os dois no stage; executável do .app 0755 */ }
func TestChecksumsComLancador(t *testing.T) { /* CHECKSUMS.sha256 tem "IA de Bolso.exe" e "IA de Bolso.app/Contents/MacOS/IA de Bolso"; LerChecksums lê os caminhos com espaço */ }
```

```js
// tests/lancadores.test.js: roda `. core/lancadores.sh; arrumar_lancadores "$tmp"` em pastas montadas pelo teste
test('lancadores.sh com os dois lançadores', ...);
test('lancadores.sh sem lançador', ...);
test('INICIAR-CD2-Mac.command roda de core/ e da raiz', ...); // com um start-unix.sh falso que grava o próprio caminho
```

- [ ] **Step 2: Rodar e ver falhar.** Rode `cd installer && go test ./internal/preparo` e `node --test tests/lancadores.test.js`. O esperado é FAIL.
- [ ] **Step 3: Implementar nos cinco lugares.** No PowerShell, use `Move-Item -LiteralPath` (os nomes têm espaço).
- [ ] **Step 4: Rodar e ver passar.** Rode `cd installer && go test ./internal/... && cd .. && node --test`. O esperado é PASS.
- [ ] **Step 5: Commit**

```bash
git add INICIAR-CD2-Windows.bat INICIAR-CD2-Mac.command core installer/internal/preparo tests/lancadores.test.js
git commit -m "Gravação: lançador na raiz do pen drive e INICIAR em core/, com plano B sem lançador"
```

---

### Task 12: CI compila o lançador e põe no ZIP

**Files:**
- Modify: `.github/workflows/zip-iadebolso.yml`

**Interfaces:**
- Consumes: `launcher/` (Task 9).
- Produces: o `cd2tech-iadebolso.zip` com `cd2tech-iadebolso/IA de Bolso.exe` e `cd2tech-iadebolso/IA de Bolso.app/` (com permissões e links preservados), antes de o manifesto calcular o SHA do ZIP.

- [ ] **Step 1: Job `lancador-mac` (`macos-14`, Go 1.23.x, Wails v2.10.1 ou o do spike).** Ele roda `wails build -clean -trimpath -platform darwin/universal -ldflags "-X main.versao=$VERSAO"`, depois `codesign --force --deep -s - "IA de Bolso.app"` e `ditto -c -k --keepParent "IA de Bolso.app" lancador-mac.zip`. Sobe o artefato `lancador-mac`.
- [ ] **Step 2: Job `lancador-windows` (`windows-latest`).** Ele roda `wails build -clean -trimpath -platform windows/amd64 -ldflags "-X main.versao=$VERSAO"` (com o WebView2 do jeito que o spike decidiu) e sobe o artefato `lancador-windows` com `IA de Bolso.exe`.
- [ ] **Step 3: O job `zip` passa a ter `needs: [lancador-mac, lancador-windows]`.** Depois do `git archive`, ele baixa os dois artefatos, extrai o `.app` com `unzip` dentro de `cd2tech-iadebolso/` e roda `zip -ry cd2tech-iadebolso.zip "cd2tech-iadebolso/IA de Bolso.app" "cd2tech-iadebolso/IA de Bolso.exe"`. Confira com `unzip -l` que os dois estão lá. Os jobs `instalador-*` continuam com `needs: zip`.
- [ ] **Step 4: Rodar no branch com `workflow_dispatch` e conferir.** O artefato `zip-e-manifesto` deve trazer os dois lançadores. No Mac, extraia o ZIP e rode `codesign -v "cd2tech-iadebolso/IA de Bolso.app"`, que não deve dar erro.
- [ ] **Step 5: Commit**

```bash
git add .github/workflows/zip-iadebolso.yml
git commit -m "CI: compila o lançador para Windows e Mac e põe no ZIP da IA de Bolso"
```

---

### Task 13: Documentação e versão 0.14.0

**Files:**
- Modify: `VERSION` (`0.14.0`), todo `?v=0.13.1` em `web/index.html` (o `tests/versao.test.js` confere)
- Modify: `README.md` (a árvore "O que tem aqui", a tabela "2. Usar" e o parágrafo do encerramento: dois cliques em `IA de Bolso`; fechar a janela deixa a IA no ícone; "Colocar a IA para dormir" encerra; o `.bat` e o `.command` ficam em `core/` como plano B)
- Modify: `docs/criar-pendrive.md` (a seção "O que deve aparecer no pen drive" e os problemas conhecidos, incluindo o aviso do sistema conforme o resultado do spike)
- Modify: `docs/index.html` (o passo "para usar")
- Modify: `.github/workflows/zip-iadebolso.yml` (as notas da release: "Para usar, dê dois cliques em IA de Bolso no pen drive")

- [ ] **Step 1: Trocar a versão e rodar o teste.** Rode `node --test tests/versao.test.js`. O esperado é PASS.
- [ ] **Step 2: Atualizar os textos.** Rode `grep -rn "INICIAR-CD2" README.md docs/criar-pendrive.md docs/index.html .github/workflows/zip-iadebolso.yml`. O esperado é que só sobrem menções como plano B ou ao Linux.
- [ ] **Step 3: Rodar todos os testes.** Rode `node --test && (cd launcher && go test ./...) && (cd installer && go test ./internal/... ./cmd/...) && (cd ctl && go test ./...)`. O esperado é PASS em tudo.
- [ ] **Step 4: Commit**

```bash
git add VERSION web/index.html README.md docs/criar-pendrive.md docs/index.html .github/workflows/zip-iadebolso.yml
git commit -m "Lançador na documentação e versão 0.14.0"
```

---

### Task 14: Prova final com pen drive de verdade

- [ ] **Step 1: Mac.** Grave um pen drive com o instalador apontando para o ZIP do artefato do branch e com o `GRAVAR-PENDRIVE-Mac.command` do ZIP extraído. Nos dois, a raiz deve mostrar `IA de Bolso.app`, `IA de Bolso.exe`, `iniciar-cd2-linux.sh` e as pastas, sem os `INICIAR` (que ficam em `core/`), e a conferência dos checksums deve passar. Repita o roteiro do Step 5 da Task 9.
- [ ] **Step 2: Windows (Gabriel).** Mesmo roteiro, com o instalador `.exe` e com o `GRAVAR-PENDRIVE-Windows.bat`. Depois de "Colocar a IA para dormir", o Gerenciador de Tarefas não deve mostrar `llamafile.exe`, `cd2-ctl.exe` nem `toolbox.exe`.
- [ ] **Step 3: Plano B.** Grave a partir de um checkout do git, sem lançadores. O `INICIAR-CD2-Mac.command` deve ficar na raiz e abrir a IA como hoje.
- [ ] **Step 4: Registrar no PR o que foi conferido em cada plataforma**, com screenshots da janelinha nos estados carregando, pronta, chamada e dormindo.
