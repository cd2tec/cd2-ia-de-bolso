# Instalar no computador pelo instalador: plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** O instalador oferece, na primeira tela, "Criar o pen drive" (grátis) ou "Instalar neste computador" (compra a cópia no Stripe, volta com a chave e instala a IA direto no computador).

**Architecture:** O caminho de instalar reaproveita o preparo do instalador (`preparo.Montar` monta o stage em `cd2-harness/usb/CD2-HARNESS`) e entrega o stage ao módulo `instalacao` (`instalacao.Instalar` com `Raiz` = stage). A chave volta do Worker para um servidor local do instalador em `127.0.0.1`, e é conferida offline antes de qualquer download. No Windows, o caminho de instalar reabre o instalador sem administrador, com `--instalar`.

**Tech Stack:** Go 1.23, Wails v2.10.1, `github.com/go-ole/go-ole` (já é dependência indireta do Wails) no Windows, JS sem build na tela (`node --test`), Worker ESM.

**Spec:** `docs/superpowers/specs/2026-10-18-instalar-pelo-instalador-design.md` (e o pai, `docs/superpowers/specs/2026-10-17-instalacao-paga-design.md`)

## Global Constraints

- Textos em português do Brasil, frases curtas, sem emoji e sem travessão.
- Cartões da primeira tela: "Criar o pen drive da IA de Bolso" com selo "Grátis"; "Instalar neste computador" com "compre a cópia, R$ 25".
- O texto da tela de condições é o do modal da conversa (`web/index.html`, `<dialog id="instalar">`, tela `compra`): sobretítulo "Cópia para este computador", título "Instale a IA de Bolso neste computador", R$ 25 "pagamento único, por instalação", listas "O que está incluído" e "Como funciona", código do computador, caixa "Entendi que a instalação vale só para este computador.", botões "Agora não" e "Ir para o pagamento", link "Já tenho a chave". Copie as frases exatas de lá.
- Chave: conferida offline com `chave.Normalizar` e `chave.Conferir(n, codigo.Ler())` **antes** de baixar. Grava com `guarda.Gravar(cacheChave, n)`.
- `cacheChave` = `cd2-harness` (`%LOCALAPPDATA%\cd2-harness`, `~/Library/Caches/cd2-harness`), a mesma do lançador e do ctl. O stage fica em `cd2-harness/usb` (`gravar.PastaCache()`), como hoje.
- Worker do build: `ambiente.Worker` (`-tags sandbox` usa o de teste). A compra é `<Worker>/comprar?codigo=<c>&porta=<p>&de=instalador`.
- Espaço antes de pagar: `preparo.BytesParaBaixar(m, usb)` + 3 GB da instalação, com folga de 10%, medidos no disco do destino.
- Nenhum `go.mod` sobe `go 1.23`, `golang.org/x/sys v0.30.0`, `howett.net/plist v1.0.1` ou `wails v2.10.1`.
- O caminho de gravar pen drive continua com o mesmo comportamento e passa nos testes de hoje, exceto pela nova tela inicial "escolha".
- Versão: `VERSION` 0.16.0 e os `?v=` do `web/index.html` acompanham.

## Review Focus

1. **Pagar e o instalador sumir ou a porta mudar:** a chave colada depois ("Já tenho a chave") instala sem novo pagamento. Teste `TestColarChaveDepoisDeReabrir` (Task 4).
2. **Chave válida mas de outro computador** colada no instalador: recusada antes de baixar, com "Esta chave não é deste computador.". Teste `TestColarChaveDeOutroComputadorNaoBaixa` (Task 4).
3. **Falta espaço:** a tela de condições avisa e não deixa ir para o pagamento. Teste `TestInfoSemEspacoBloqueiaPagamento` (Task 4) e `condicoes sem espaço` (Task 5).
4. **Página local de volta chamada com lixo** (`/?chave=abc`, sem chave, outro caminho): não derruba nada, não aceita, e responde em português. Teste `TestVoltaIgnoraChaveRuim` (Task 2).
5. **Gravar pen drive depois de instalar** (ou o contrário) na mesma janela: os dois caminhos não se misturam, e "Gravar outro" volta ao pen drive. Teste `outro volta para o pen drive` (Task 5).

---

### Task 1: Worker: `de=instalador`

**Files:**
- Modify: `worker/src/index.js`, `worker/src/paginas.js`
- Test: `tests/worker.test.mjs`

**Interfaces:**
- Produces: `/comprar?...&de=instalador`. Aceita só o valor exato `instalador`, e qualquer outro é ignorado. Grava `metadata[de]=instalador` e usa `cancel_url = BASE_URL + '/cancelou?de=instalador'`. Em `/voltou`, com `metadata.de === 'instalador'`, o botão diz "Voltar para o instalador". Em `/cancelou?de=instalador`, o texto é "Tudo bem, nada foi cobrado. Pode fechar esta página e voltar para o instalador.". Sem o parâmetro, tudo fica como hoje.

- [ ] **Passo 1: Testes que falham:**
  - `comprarDeInstaladorGravaMetadata`: corpo do Stripe com `metadata[de]` = `instalador` e `cancel_url` terminando em `/cancelou?de=instalador`.
  - `comprarIgnoraDeDesconhecido`: `de=x` não grava `metadata[de]` e o `cancel_url` termina em `/cancelou`.
  - `voltouDeInstaladorMudaBotao`: a página contém "Voltar para o instalador" e não contém "Voltar para a IA".
  - `cancelouDeInstalador`: a página contém o texto novo.
- [ ] **Passo 2:** `node --test tests/worker.test.mjs`. Expected: FAIL.
- [ ] **Passo 3: Implementar.**
- [ ] **Passo 4:** `node --test`. Expected: PASS em tudo.
- [ ] **Passo 5: Commit** `Worker: compra vinda do instalador volta para o instalador`. Publicar nos dois ambientes (`npx wrangler@4 deploy` e `npx wrangler@4 deploy --env sandbox`) fica com o controller, depois da revisão.

---

### Task 2: Instalador: servidor local da volta

**Files:**
- Modify: `installer/go.mod` (`require github.com/cd2tec/cd2-ia-de-bolso/instalacao v0.0.0` + `replace ... => ../instalacao`, copiando as linhas do `go.sum` como o ctl fez)
- Create: `installer/internal/volta/volta.go`, `installer/internal/volta/volta_test.go`

**Interfaces:**
- Produces: `volta.Ouvir(ctx context.Context) (*volta.Servidor, error)`, que escuta em `127.0.0.1:0`. `(*Servidor).Porta() int` e `(*Servidor).Chaves() <-chan string` entregam a chave já normalizada, uma vez. `(*Servidor).Fechar()`.
- `GET /?chave=<x>`: se `chave.Normalizar` aceitar, entrega no canal e responde 200 com HTML simples: "Pronto, a chave chegou. Pode fechar esta página e voltar para o instalador.". Senão, responde 400 com "Esta chave não está completa. Copie a chave inteira da página do pagamento e cole no instalador.". Qualquer outro caminho dá 404. Só aceita `Host` `127.0.0.1:<porta>` ou `localhost:<porta>`. Responde com `Cache-Control: no-store`.

- [ ] **Passo 1: Testes que falham:**
  - `TestVoltaEntregaChaveNormalizada`: colagem suja, usando a chave do vetor `instalacao/testdata/vetor.json`, chega normalizada e responde 200.
  - `TestVoltaIgnoraChaveRuim`: `/?chave=abc`, `/` sem chave e `/outro` respondem 400, 400 e 404, sem nada no canal.
  - `TestVoltaRecusaOutroHost`.
- [ ] **Passo 2:** `cd installer && go test ./internal/volta/`. Expected: FAIL.
- [ ] **Passo 3: Implementar.**
- [ ] **Passo 4:** `cd installer && test -z "$(gofmt -l .)" && go vet ./internal/... && GOOS=windows go vet ./internal/... && go test ./internal/...`. Expected: PASS.
- [ ] **Passo 5: Commit** `Instalador: servidor local que recebe a chave do pagamento`.

---

### Task 3: Instalador: instalar no computador a partir do stage

**Files:**
- Create: `installer/internal/gravar/computador.go`, `installer/internal/gravar/computador_test.go`

**Interfaces:**
- Consumes: `manifesto.Obter`, `preparo.Montar`, `preparo.BytesParaBaixar`, o `andamento` já existente em `gravar.go`, `instalacao.Instalar`, `instalacao.Opcoes`, `instalacao.ErrAvisoAtalhos`, `copiar.EspacoLivre`, `sistema.Destino`.
- Produces:
  - `type OpcoesComputador struct { Cache, CacheChave, URLManifesto string; HTTP *http.Client; AreaDeTrabalho, Inicio bool; Agora func() time.Time }`.
  - `func InstalarNoComputador(ctx context.Context, o OpcoesComputador, ev func(Evento)) error`. Emite os mesmos `Evento`s do gravar, com as etapas `"baixando"`, `"conferindo"`, `"instalando"` e `"pronto"`, numa fração só. Erros saem como `*Erro` com frase em português. Com `ErrAvisoAtalhos`, devolve nil e o `Evento` "pronto" leva a frase do aviso.
  - `func EspacoParaInstalar(ctx context.Context, o OpcoesComputador) (precisa, livre int64, err error)`. `precisa = (BytesParaBaixar + 3_000_000_000) * 1.1`, e `livre` = `copiar.EspacoLivre` no pai do destino.
  - As variáveis de pacote trocáveis `instalarFn = instalacao.Instalar` e `espacoLivreFn = copiar.EspacoLivre` existem para os testes.

- [ ] **Passo 1: Testes que falham**, com o servidor de manifesto falso que os testes do gravar já usam:
  - `TestInstalarNoComputadorMontaEInstala`: `instalarFn` recebe `Raiz` = `<cache>/CD2-HARNESS`, `Cache` = `CacheChave` e as duas opções. O último evento é "pronto", com fração 1.
  - `TestInstalarNoComputadorAvisoNaoEhErro`: `instalarFn` devolve `errors.Join(instalacao.ErrAvisoAtalhos, x)`. O resultado é nil e o "pronto" traz o texto do aviso.
  - `TestInstalarNoComputadorSemChave`: `instalacao.ErrSemChave` vira `*Erro` com a frase "Falta a chave deste computador.".
  - `TestEspacoParaInstalar`: o `precisa` bate com a fórmula.
- [ ] **Passo 2:** `cd installer && go test ./internal/gravar/ -run Computador`. Expected: FAIL.
- [ ] **Passo 3: Implementar**, reaproveitando o `andamento`. O peso da etapa "instalando" é o total do stage, e o `prog` do `instalacao.Instalar` alimenta essa etapa.
- [ ] **Passo 4:** `cd installer && go vet ./... && GOOS=windows go vet ./internal/... && go test ./...`. Expected: PASS, com o gravar de hoje intacto.
- [ ] **Passo 5: Commit** `Instalador: instala a IA no computador a partir do stage`.

---

### Task 4: Instalador: Windows sem administrador e ponte com a tela

**Files:**
- Create: `installer/internal/elevacao/elevacao.go`, `installer/internal/elevacao/elevacao_windows.go`, `installer/internal/elevacao/elevacao_outros.go`, `installer/internal/elevacao/elevacao_test.go`
- Create: `installer/computador.go`, `installer/computador_test.go`
- Modify: `installer/main.go` (lê `--instalar` e passa para a App)

**Interfaces:**
- `elevacao.Elevado() bool`: no Windows, é o token do processo elevado (`windows.GetCurrentProcessToken().IsElevated()`). Fora dele, é sempre false.
- `elevacao.ReabrirSemAdmin(exe string, args []string) error`: no Windows, pede ao shell do usuário logado que abra o programa, pela técnica IShellWindows → janela do desktop → IShellDispatch2.ShellExecute, com go-ole. Fora do Windows, devolve um erro.
- `elevacao.LerArgs([]string) bool` diz se veio `--instalar` e ignora o resto.
- Métodos Wails na App:
  - `InfoInstalar() InfoInstalar`, com `type InfoInstalar struct { Codigo string; TemChave bool; Worker string; Sandbox bool; Precisa, Livre int64; Cabe bool; Bloqueio string }` (json em minúsculas). `Bloqueio` só tem valor quando não deu para reabrir sem administrador: "Abra o instalador de novo sem ser como administrador (clique duplo normal) para instalar neste computador.".
  - `EscolherInstalar() string`: no Windows elevado, chama `ReabrirSemAdmin(os.Executable(), ["--instalar"])`. Se der certo, sai do app e devolve "". Se der errado, devolve o texto do `Bloqueio`. Nos outros casos devolve "".
  - `IrParaPagamento() error`: sobe `volta.Ouvir`, abre `<Worker>/comprar?codigo=..&porta=..&de=instalador` com `runtime.BrowserOpenURL` e, quando a chave chegar, chama `ColarChave` e emite o evento `"chave"` com `{ok, erro}`.
  - `ColarChave(k string) string`: normaliza e confere com o código do computador, devolvendo "" se a chave valer. Os erros são "Esta chave não está completa. Confira se copiou ela inteira." e "Esta chave não é deste computador.". Grava com `guarda.Gravar`.
  - `InstalarComputador(areaDeTrabalho, inicio bool) error`: roda `gravar.InstalarNoComputador` em goroutine e emite `"progresso"` e `"fim"`, como o `Gravar`. Usa o mesmo `cancelar` e o mesmo `antesDeFechar`, com a mensagem "A instalação ainda não terminou. Fechar mesmo assim?".
  - `AbrirIA() error`: inicia `sistema.Lancador(destino, GOOS)` destacado, com `Dir` = pasta do lançador, e sai do app.
  - `IniciouComInstalar() bool`.

- [ ] **Passo 1: Testes que falham** (`installer/computador_test.go`, com `codigo`, `guarda` e espaço trocáveis):
  - `TestColarChaveDeOutroComputadorNaoBaixa`: devolve "Esta chave não é deste computador.", nada é gravado e a função de instalar não é chamada.
  - `TestColarChaveDepoisDeReabrir`: numa App nova, a chave do vetor colada com o código do vetor é aceita e gravada, e `InfoInstalar().TemChave` passa a true.
  - `TestInfoSemEspacoBloqueiaPagamento`: livre menor que precisa dá `Cabe` false.
  - `TestURLDeCompra`: `<Worker>/comprar?codigo=BCNK-0X06-DV8S&porta=5000&de=instalador`.
  - `TestLerArgs`: em `elevacao_test.go`.
- [ ] **Passo 2:** `cd installer && go test . ./internal/elevacao/`. Expected: FAIL.
- [ ] **Passo 3: Implementar.** O `elevacao_windows.go` só precisa compilar e passar no vet aqui. O teste real é manual (Task 7).
- [ ] **Passo 4:** `cd installer && test -z "$(gofmt -l .)" && go vet ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./internal/... && go test ./... && go test -tags sandbox ./...`. Expected: PASS.
- [ ] **Passo 5: Commit** `Instalador: escolher instalar, pagar, colar a chave e instalar (ponte Go)`.

---

### Task 5: Instalador: as telas

**Files:**
- Modify: `installer/frontend/dist/fluxo.js` (estados novos), `installer/frontend/dist/tela.js` (desenho e ligações), `installer/frontend/dist/index.html`, `installer/frontend/dist/estilo.css`
- Test: `tests/instalador-fluxo.test.js`

**Interfaces:**
- Consumes: os métodos da Task 4 e os eventos `"progresso"`, `"fim"` e `"chave"`.
- `fluxo.js`:
  - `inicial()` passa a devolver `{ tela: 'escolha', ... }`.
  - A ação `escolherPendrive` leva a `'plugue'`, e o resto do pen drive fica igual.
  - A ação `escolherInstalar` leva a `'condicoes'`, ou a `'reinstalar'` se `temChave`.
  - Telas novas: `'condicoes'`, `'aguardando'`, `'colar'`, `'reinstalar'`, `'perguntas'`, `'instalando'`, `'instalado'`, `'bloqueado'`.
  - `outro`, a partir de `pronto` ou `erro` do pen drive, volta para `'plugue'`, não para `'escolha'`.
  - `etapa(tela)` só numera as telas do pen drive. Para as outras, a barra de etapas some.
- Textos: Global Constraints e spec, seção "A experiência". Na tela de condições, a área rola dentro da janela de 560x420.

- [ ] **Passo 1: Testes que falham** em `tests/instalador-fluxo.test.js`:
  - `inicial é escolha`;
  - `escolherPendrive vai para plugue`;
  - `escolherInstalar com chave vai para reinstalar`;
  - `condicoes sem espaço não deixa pagar` (a ação `pagar` é ignorada com `cabe:false`);
  - `pagar só com Entendi marcado`;
  - `chave ok vai para perguntas`;
  - `instalar e fim ok vai para instalado`;
  - `outro volta para o pen drive`;
  - ajustar os testes antigos que esperavam `'plugue'` no `inicial()`.
- [ ] **Passo 2:** `node --test tests/instalador-fluxo.test.js`. Expected: FAIL.
- [ ] **Passo 3: Implementar.** No modo demonstração do `tela.js` (sem Wails), o backend falso responde aos métodos novos.
- [ ] **Passo 4:** `node --test`. Expected: PASS. Conferir as telas no navegador abrindo `installer/frontend/dist/index.html` em modo demonstração, com captura das telas escolha, condicoes, instalando e instalado.
- [ ] **Passo 5: Commit** `Instalador: telas de escolher, comprar e instalar no computador`.

---

### Task 6: Documentação e versão

**Files:**
- Modify: `VERSION` (0.16.0), os `?v=` de `web/index.html`, `README.md` (a seção "Instalar no computador" ganha o caminho pelo instalador), `docs/criar-pendrive.md` (a primeira tela tem duas escolhas), `docs/index.html` (texto do download: o instalador grava o pen drive grátis ou instala no computador)

- [ ] **Passo 1:** Atualizar a versão e os `?v=`. Rodar `node --test`. Expected: PASS (inclui `tests/versao.test.js`).
- [ ] **Passo 2:** Atualizar os textos nas regras do projeto.
- [ ] **Passo 3: Commit** `Instalador instala no computador; versão 0.16.0`.

---

### Task 7: Conferência manual (com o Gabriel)

Usar o instalador compilado com `-tags sandbox`, apontando para o Worker de teste e o cartão `4242 4242 4242 4242`.

- [ ] **Mac:** escolher "Instalar neste computador", pagar, voltar com a chave, ver o download, instalar, "Abrir a IA de Bolso" e reiniciar.
- [ ] **Windows, administrador sendo o próprio usuário:** escolher instalar. O instalador reabre sem administrador direto nas condições, e o resto do fluxo segue igual ao do Mac. A IA deve ficar em `%LOCALAPPDATA%\Programs` da conta logada.
- [ ] **Windows, outra conta digitando a senha de administrador:** mesmo teste, e a IA fica na conta de quem está usando. Se a reabertura falhar, aparece o bloqueio, e o caminho do pen drive continua funcionando.
- [ ] **Fechar o instalador no meio do pagamento:** reabrir, usar "Já tenho a chave" e instalar.
- [ ] **Gravar pen drive:** o caminho de sempre continua igual.
