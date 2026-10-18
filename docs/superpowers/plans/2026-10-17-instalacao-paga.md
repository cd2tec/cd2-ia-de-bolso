# Instalação paga no computador: plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Quem testa a IA de Bolso pelo pen drive paga R$ 25 no Stripe e instala a IA naquele computador, que passa a abrir sem o pen drive e só nele.

**Architecture:** Um módulo Go novo, `instalacao/`, concentra o código do computador, a chave Ed25519, a regra de pen drive ou disco, a cópia e os atalhos. O `cd2-ctl` usa esse módulo para dar à conversa as rotas da instalação, e o lançador usa o mesmo módulo para a trava, o menu do ícone e o desinstalar. Um Worker na Cloudflare (`worker/`) cria o Checkout do Stripe e assina a chave, sem guardar nada.

**Tech Stack:** Go 1.23 (`crypto/ed25519`, `golang.org/x/sys`), Wails v2 e `fyne.io/systray` (já no lançador), JavaScript sem build (web e janelinha, testes com `node --test`), Cloudflare Workers com WebCrypto Ed25519 e a API REST do Stripe via `fetch`, sem SDK.

**Spec:** `docs/superpowers/specs/2026-10-17-instalacao-paga-design.md`

## Global Constraints

- Textos em português do Brasil, frases curtas, sem emoji e sem travessão (regra do README).
- Preço: R$ 25, BRL, preço cadastrado no Stripe (`STRIPE_PRICE_ID`). O texto do botão nunca mostra o preço; o modal mostra "R$ 25" e "pagamento único, por instalação".
- Endereço do Worker: `https://instalariadebolso.cd2.com.br`, gravado como constante única no web (`web/instalar.js`).
- Mensagem assinada: `cd2-instalacao-v1|<código>`. Chave para a pessoa: `CD2-` + assinatura Ed25519 de 64 bytes em base32 Crockford maiúsculo, sem preenchimento (103 caracteres), 107 no total.
- Código do computador: SHA-256 de `cd2-iadebolso-codigo-v1|` + identidade em minúsculas e sem espaços nas pontas; os 60 primeiros bits em base32 Crockford (`0123456789ABCDEFGHJKMNPQRSTVWXYZ`), 12 caracteres, mostrados como `XXXX-XXXX-XXXX`.
- A chave mora em `<cache>/instalacao.chave`, com permissão 0600. O cache é o de hoje (`%LOCALAPPDATA%\cd2-harness`, `~/Library/Caches/cd2-harness`), fora da pasta de instalação.
- Destino da instalação: Windows `%LOCALAPPDATA%\Programs\CD2 IA de Bolso`; Mac `~/Applications/IA de Bolso/`, com o `IA de Bolso.app` dentro e os arquivos ao lado.
- Sem administrador em nenhum passo. Nenhum segredo (chave do Stripe, chave privada Ed25519) no repositório: só como *secret* do Worker.
- Tudo é construído e testado no **modo de teste** do Stripe. O modo real só é ligado depois da resposta da contadora (fora deste plano).
- Versão: sobe a menor do `VERSION` no merge (hoje 0.14.0, vai para 0.15.0, ou a seguinte se outra branch subir antes). O `?v=` dos scripts no `web/index.html` acompanha.
- Mudou o `cd2-ctl`: o release `ctl-v<versão>` novo é obrigatório, e a versão fixada nos scripts de preparo e no manifesto do instalador sobe junto.

## Review Focus

1. **O teste pelo pen drive nunca pode trancar.** Pen drive que o Windows vê como disco fixo, Mac com o pen drive montado num caminho estranho ou ferramenta do sistema que falha: na dúvida, `origem.Detectar` responde `Pendrive`. Teste em `origem_test.go`, `TestNaDuvidaEPendrive` (Task 4).
2. **Pagou e a volta não achou a IA** (porta mudou, aba fechada, IA fechada): a página do Worker sempre mostra a chave, e o modal sempre aceita colar. Teste em `tests/instalar.test.js`, `voltaSemAbaOriginal` (Task 8), e em `tests/worker.test.mjs`, `voltouMostraChaveMesmoSemPorta` (Task 3).
3. **Chave colada com espaço, quebra de linha, minúsculas ou hífens a mais:** é aceita. Teste `TestNormalizarAceitaColagemSuja` (Task 1).
4. **Faltou espaço ou a cópia falhou no meio:** nenhuma meia instalação fica para trás, e uma instalação anterior continua de pé. Teste `TestCopiaQueFalhaNaoEstragaAnterior` (Task 5).
5. **Desinstalar com a IA acordada no Windows** (o `.exe` em uso não pode ser apagado): a pasta some depois que o lançador sai. Teste `TestPlanoDeRemocaoWindowsAgendaDepoisDeSair` (Task 6).

---

## Mapa de arquivos

- `instalacao/` (módulo novo `github.com/cd2tec/cd2-ia-de-bolso/instalacao`)
  - `codigo/`: código do computador (`Derivar`, `Ler`).
  - `chave/`: formato, normalização e conferência da chave; a chave pública embutida.
  - `guarda/`: ler e gravar `instalacao.chave` no cache.
  - `origem/`: pen drive ou disco.
  - `copiar/`: o que copiar, conferência com o `CHECKSUMS.sha256`, troca atômica.
  - `sistema/`: atalhos, início automático e registro de desinstalação, por sistema.
  - `instalar.go`: `Instalar` e `Desinstalar`, que juntam tudo.
  - `testdata/vetor.json`: vetor compartilhado com o Worker.
- `worker/`: `src/index.js` (rotas), `src/chave.js` (código e assinatura), `src/paginas.js` (HTML), `wrangler.toml`, `gerar-chaves.mjs`, `LEIAME.md`.
- `ctl/internal/api/instalacao.go`: rotas `/instalacao*`. `ctl/main.go`: liga o backend.
- `launcher/`: `trava.go`, `instalada.go`, mudanças em `main.go`, `janela.go`, `bandeja.go` e `frontend/dist/tela.js`.
- `web/instalar.js` (lógica sem DOM), `web/index.html` (etiqueta, botão, `<dialog id="instalar">`).
- `docs/condicoes-instalacao.html`, `docs/criar-pendrive.md`, `README.md`, `VERSION`.
- Testes: `instalacao/**/_test.go`, `ctl/internal/api/instalacao_test.go`, `launcher/*_test.go`, `tests/instalar.test.js`, `tests/worker.test.mjs`, `tests/lancador-tela.test.js`.

---

### Task 0: Conferir os riscos antes de construir o resto (manual, com o Gabriel)

Estes passos rodam em máquinas reais e não bloqueiam as Tasks 1 a 3, mas bloqueiam as Tasks 5, 6 e 10 a 12 se der errado. Anote o resultado de cada passo em `docs/superpowers/plans/2026-10-17-instalacao-paga-riscos.md`.

- [ ] **Passo 1: Pen drive que parece disco fixo (risco 1).** Em três pen drives de marcas diferentes no Windows, rode `Get-Disk | Select Number,BusType,FriendlyName` e `(Get-Volume -DriveLetter E).DriveType`. Anote quais aparecem com `DriveType = Fixed` e `BusType = USB`. A regra da Task 4 trata `BusType = USB` como pen drive, mesmo com `Fixed`. Se algum pen drive real aparecer com outro `BusType`, pare e traga ao Gabriel.
- [ ] **Passo 2: `.app` e `.exe` copiados para o disco (risco 2).** Copie à mão `IA de Bolso.app` de um pen drive gravado para `~/Applications/IA de Bolso/` e abra com dois cliques. Rode `codesign --verify --deep --strict` e `spctl -a -vv` no `.app` copiado. No Windows, copie `IA de Bolso.exe` para `%LOCALAPPDATA%\Programs\CD2 IA de Bolso\` e abra. Anote se aparece aviso. Com aviso, o texto de "como passar por ele" entra na Task 13.
- [ ] **Passo 3: Launchpad e Spotlight.** Com o `.app` em `~/Applications/IA de Bolso/`, confira se ele aparece no Spotlight e no Launchpad. Se não aparecer no Launchpad, a Task 6 cria também um link simbólico `~/Applications/IA de Bolso.app` que aponta para o `.app` da pasta.
- [ ] **Passo 4: A chave na fatura do Stripe (risco 3).** No modo de teste, crie à mão uma Checkout Session com `invoice_creation[enabled]=true` e um `custom_fields` de 107 caracteres. Confira que é aceita e que a fatura do e-mail de teste mostra o valor inteiro. Anote o custo da fatura pós-pagamento no painel. Se for recusada, pare e traga ao Gabriel: a alternativa (Resend) precisa do aval dele.
- [ ] **Passo 5: Pix (risco 4).** Veja no painel do Stripe da CD2 se o Pix está disponível. O plano não depende disso: o Checkout usa os meios automáticos do painel.

---

### Task 1: Módulo `instalacao`: código do computador e chave

**Files:**
- Create: `instalacao/go.mod`, `instalacao/codigo/codigo.go`, `instalacao/codigo/ler_windows.go`, `instalacao/codigo/ler_darwin.go`, `instalacao/codigo/ler_outros.go`, `instalacao/codigo/codigo_test.go`
- Create: `instalacao/chave/chave.go`, `instalacao/chave/publica.go`, `instalacao/chave/chave_test.go`, `instalacao/testdata/vetor.json`
- Modify: `.github/workflows/ci.yml` (job `instalacao`, igual ao `launcher` sem o `go vet .`)

**Interfaces:**
- Produces:
  - `codigo.Derivar(identidade string) string`: devolve `XXXX-XXXX-XXXX`.
  - `codigo.Ler() (string, error)`: identidade da máquina derivada. No Windows, `MachineGuid` de `HKLM\SOFTWARE\Microsoft\Cryptography`; no Mac, `IOPlatformUUID` lido de `ioreg -rd1 -c IOPlatformExpertDevice`; em outro sistema, erro.
  - `chave.Mensagem(codigo string) []byte`: `cd2-instalacao-v1|` + código.
  - `chave.Formatar(sig []byte) string`: `CD2-` + base32 Crockford.
  - `chave.Normalizar(s string) (string, bool)`: tira espaços, quebras de linha e hífens; passa para maiúsculas; troca `O→0`, `I→1` e `L→1`; exige o prefixo `CD2` mais 103 caracteres válidos; devolve no formato de `Formatar`.
  - `chave.ConferirCom(pub ed25519.PublicKey, chave, codigo string) bool`.
  - `chave.Conferir(chave, codigo string) bool`, que usa `chave.Publica`.
  - `chave.Publica ed25519.PublicKey`: em `publica.go`, preenchida na Task 2, passo 5.
- `instalacao/testdata/vetor.json`: `{"privadaPKCS8Base64": "...", "publicaBase64": "...", "identidade": "...", "codigo": "...", "chave": "CD2-..."}`, gerado uma vez e só para teste. É usado aqui e no Worker.

- [ ] **Passo 1: Escrever os testes que falham**

```go
func TestDerivarFormato(t *testing.T) {
	c := codigo.Derivar("  4C4C4544-0042-3510-8051-B4C04F4B4E32 ")
	if !regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{4}-[0-9A-HJKMNP-TV-Z]{4}-[0-9A-HJKMNP-TV-Z]{4}$`).MatchString(c) { t.Fatal(c) }
	if c != codigo.Derivar("4c4c4544-0042-3510-8051-b4c04f4b4e32") { t.Fatal("maiúsculas e espaços mudaram o código") }
}
func TestVetorCompartilhado(t *testing.T) { // lê ../testdata/vetor.json
	if codigo.Derivar(v.Identidade) != v.Codigo { t.Fatal("código") }
	if !chave.ConferirCom(pub, v.Chave, v.Codigo) { t.Fatal("chave do vetor") }
	if chave.ConferirCom(pub, v.Chave, "AAAA-AAAA-AAAA") { t.Fatal("aceitou outro computador") }
}
func TestFormatarTamanho(t *testing.T) { if n := len(chave.Formatar(make([]byte, 64))); n != 107 { t.Fatal(n) } }
func TestNormalizarAceitaColagemSuja(t *testing.T) {
	sujo := " cd2-" + strings.ToLower(v.Chave[4:20]) + "\n-" + v.Chave[20:] + "  "
	got, ok := chave.Normalizar(sujo)
	if !ok || got != v.Chave { t.Fatal(got) }
	if _, ok := chave.Normalizar("CD2-curta"); ok { t.Fatal("aceitou chave curta") }
}
```

- [ ] **Passo 2: Gerar o vetor** com um `go run` descartável (não commitado): par Ed25519 novo, identidade `4C4C4544-0042-3510-8051-B4C04F4B4E32`, assinatura de `Mensagem(Derivar(identidade))`. Grave em `instalacao/testdata/vetor.json`.
- [ ] **Passo 3: Rodar e ver falhar.** Run: `cd instalacao && go test ./...`. Expected: FAIL (pacotes vazios).
- [ ] **Passo 4: Implementar** as funções dos Interfaces. `ConferirCom` normaliza, decodifica a base32 e chama `ed25519.Verify`. Por enquanto, `publica.go` declara `var Publica ed25519.PublicKey`, vazia, e `Conferir` devolve `false` com ela vazia. `ler_*.go` com build tags.
- [ ] **Passo 5: Rodar e ver passar.** Run: `cd instalacao && go test ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./...`. Expected: PASS, sem saída de vet.
- [ ] **Passo 6: CI.** Adicionar o job `instalacao` ao `ci.yml` (gofmt, vet nos três GOOS, test).
- [ ] **Passo 7: Commit** `git add instalacao .github/workflows/ci.yml && git commit -m "Instalação: código do computador e chave Ed25519"`

---

### Task 2: Worker: assinatura em JavaScript e chaves de produção

**Files:**
- Create: `worker/src/chave.js` (ESM), `worker/gerar-chaves.mjs`, `worker/package.json` (`"type": "module"`, só `wrangler` em devDependencies), `tests/worker.test.mjs`
- Modify: `instalacao/chave/publica.go`

**Interfaces:**
- Consumes: `instalacao/testdata/vetor.json` (Task 1).
- Produces (de `worker/src/chave.js`):
  - `codigoValido(s: string): boolean`: regex do formato `XXXX-XXXX-XXXX`.
  - `assinar(codigo: string, privadaPKCS8Base64: string): Promise<string>`: chave no formato `CD2-...`, com WebCrypto `{name: 'Ed25519'}`.

- [ ] **Passo 1: Escrever o teste que falha** em `tests/worker.test.mjs`:

```js
test('assinar reproduz o vetor do Go', async () => {
  const v = JSON.parse(readFileSync('instalacao/testdata/vetor.json', 'utf8'));
  assert.equal(await assinar(v.codigo, v.privadaPKCS8Base64), v.chave);
});
test('codigoValido', () => {
  assert.ok(codigoValido('7F3K-Q9WD-2MXA'));
  assert.ok(!codigoValido('7F3K-Q9WD-2MXI')); // I não existe no Crockford
  assert.ok(!codigoValido('7F3KQ9WD2MXA'));
});
```

- [ ] **Passo 2: Rodar e ver falhar.** Run: `node --test tests/worker.test.mjs`. Expected: FAIL (módulo não existe).
- [ ] **Passo 3: Implementar `worker/src/chave.js`.** A base32 Crockford é a mesma da Task 1. Ed25519 é determinístico, então o resultado é idêntico ao do Go.
- [ ] **Passo 4: Rodar e ver passar.** Run: `node --test`. Expected: PASS em tudo.
- [ ] **Passo 5: Gerar o par de produção.** `worker/gerar-chaves.mjs` imprime a privada PKCS8 em base64, com a instrução `wrangler secret put CHAVE_PRIVADA`, e a pública em base64. **O Gabriel roda o script uma vez** e guarda a privada fora do repo. A pública vai para `instalacao/chave/publica.go` com um comentário sobre a origem.
- [ ] **Passo 6: Commit** `git commit -m "Worker: assinatura da chave e chave pública de produção"`

---

### Task 3: Worker: rotas, Stripe e páginas

**Files:**
- Create: `worker/src/index.js`, `worker/src/paginas.js`, `worker/wrangler.toml`, `worker/LEIAME.md`
- Test: `tests/worker.test.mjs`

**Interfaces:**
- Consumes: `assinar`, `codigoValido` (Task 2).
- Produces: as rotas abaixo, consumidas pelo web (Task 8):
  - `GET /comprar?codigo=<código>&porta=<1024..65535>`: 302 para a URL do Checkout. Código inválido ou porta fora da faixa: 400, com página em português.
  - `GET /voltou?session_id=<id>`: 200 com a página da chave, se a sessão estiver paga. Sessão não paga ou desconhecida: 402 com "O pagamento ainda não foi confirmado", sem chave.
  - `GET /cancelou`: 200, "Tudo bem, a IA continua no pen drive."
- `env`: `STRIPE_SECRET_KEY`, `STRIPE_PRICE_ID`, `CHAVE_PRIVADA` (secrets) e `BASE_URL = "https://instalariadebolso.cd2.com.br"` (var).
- `export default { fetch(req, env) }` e `export function criar({ fetch }) { return { fetch(req, env) } }`, para o teste trocar o `fetch` do Stripe.

O Checkout é criado com `POST https://api.stripe.com/v1/checkout/sessions`, em form-urlencoded, com estes campos:

| Campo | Valor |
|---|---|
| `mode` | `payment` |
| `line_items[0][price]` | `STRIPE_PRICE_ID` |
| `line_items[0][quantity]` | `1` |
| `client_reference_id` | o código |
| `metadata[porta]` | a porta |
| `success_url` | `BASE_URL + '/voltou?session_id={CHECKOUT_SESSION_ID}'` |
| `cancel_url` | `BASE_URL + '/cancelou'` |
| `locale` | `pt-BR` |
| `invoice_creation[enabled]` | `true` |
| `invoice_creation[invoice_data][custom_fields][0][name]` | `Chave de instalação` |
| `invoice_creation[invoice_data][custom_fields][0][value]` | a chave |
| `consent_collection[terms_of_service]` | `required` |

- [ ] **Passo 1: Escrever os testes que falham**, com um `fetch` falso que grava os pedidos e responde como o Stripe:

```js
test('comprarCriaCheckoutComCodigoEChave', async () => {
  const r = await app.fetch(new Request(BASE + '/comprar?codigo=' + v.codigo + '&porta=8080'), env);
  assert.equal(r.status, 302);
  const corpo = new URLSearchParams(pedidos[0].body);
  assert.equal(corpo.get('client_reference_id'), v.codigo);
  assert.equal(corpo.get('invoice_creation[invoice_data][custom_fields][0][value]'), v.chave);
  assert.equal(corpo.get('metadata[porta]'), '8080');
});
test('comprarRecusaCodigoInvalido', ...); // 400, nenhum pedido ao Stripe
test('voltouPagaMostraChaveEVolta', ...); // 200, corpo contém v.chave e 'http://127.0.0.1:8080/?chave=' + encodeURIComponent(v.chave)
test('voltouMostraChaveMesmoSemPorta', ...); // metadata sem porta: 200 com a chave, sem botão de voltar
test('voltouNaoPagaNaoMostraChave', ...); // payment_status 'unpaid': 402, corpo sem 'CD2-'
```

- [ ] **Passo 2: Rodar e ver falhar.** Run: `node --test tests/worker.test.mjs`. Expected: FAIL.
- [ ] **Passo 3: Implementar** `index.js` e `paginas.js`.
  - Identidade da CD2 inline: Manrope pelo Google Fonts, `#0000ff`, `#f2f2f2`, botões em pílula.
  - A página da chave tem "Copiar a chave" e "Voltar para a IA" (`http://127.0.0.1:<porta>/?chave=<chave>`). Com a porta, faz a volta automática em 3 s.
  - Toda resposta leva `Cache-Control: no-store`.
- [ ] **Passo 4: Rodar e ver passar.** Run: `node --test`. Expected: PASS.
- [ ] **Passo 5: `wrangler.toml` e `LEIAME.md`.** `wrangler.toml` com `routes = [{ pattern = "instalariadebolso.cd2.com.br", custom_domain = true }]`. O `LEIAME.md` explica o passo a passo da CD2:
  1. Criar o preço de R$ 25 no modo de teste.
  2. Cadastrar a URL das condições do serviço (Task 13) no painel do Stripe, exigida pelo `consent_collection`.
  3. `wrangler secret put` das três chaves.
  4. `wrangler deploy`.
- [ ] **Passo 6: Teste de ponta a ponta no modo de teste.** Publicar, abrir `/comprar?codigo=7F3K-Q9WD-2MXA&porta=8080`, pagar com o cartão `4242 4242 4242 4242` e conferir a página da chave, o e-mail de teste e a fatura com o campo.
- [ ] **Passo 7: Commit** `git commit -m "Worker: Checkout do Stripe, volta com a chave e páginas"`

---

### Task 4: `instalacao`: onde a chave mora e de onde a IA roda

**Files:**
- Create: `instalacao/guarda/guarda.go`, `instalacao/guarda/guarda_test.go`
- Create: `instalacao/origem/origem.go`, `instalacao/origem/origem_windows.go`, `instalacao/origem/origem_darwin.go`, `instalacao/origem/origem_outros.go`, `instalacao/origem/origem_test.go`

**Interfaces:**
- Consumes: `chave.Normalizar`, `chave.Conferir`, `codigo.Ler` (Task 1).
- Produces:
  - `guarda.Ler(cache string) string`: a chave normalizada, ou `""`.
  - `guarda.Gravar(cache, chave string) error`: grava a chave normalizada com permissão 0600 e cria a pasta.
  - `guarda.Valida(cache string) bool`: `Conferir(Ler(cache), codigo.Ler())`.
  - `type Origem string`, com as constantes `Pendrive`, `Instalada` e `Desconhecida`.
  - `origem.Detectar(raiz string) Origem`, que usa `origem.Regra(info Volume) Origem` (pura) e `lerVolume(raiz) (Volume, error)`, esta por sistema.
  - `type Volume struct { DoSistema, Removivel, USB, Externo bool }`.

A regra é escolha deste plano, dentro do que o spec deixa a quem constrói:

- `DoSistema` → `Instalada`.
- `Removivel || USB || Externo` → `Pendrive`.
- Senão → `Instalada`.
- Erro ao ler o volume → `Pendrive`. Na dúvida, nunca tranca.

Leitura do volume:
- **Windows:** `DoSistema` compara a letra com `%SystemDrive%`. `Removivel` vem do `GetDriveTypeW`. `USB` vem do `BusType` do disco, por `IOCTL_STORAGE_QUERY_PROPERTY` no volume.
- **Mac:** `diskutil info -plist <ponto de montagem>`, com `Internal == false` → `Externo`, `RemovableMedia` → `Removivel` e o ponto de montagem `/` → `DoSistema`.

- [ ] **Passo 1: Escrever os testes que falham**

```go
func TestRegra(t *testing.T) {
	casos := []struct{ v origem.Volume; quer origem.Origem }{
		{origem.Volume{DoSistema: true}, origem.Instalada},
		{origem.Volume{USB: true}, origem.Pendrive},           // pen drive "fixo" do Windows
		{origem.Volume{Externo: true}, origem.Pendrive},
		{origem.Volume{}, origem.Instalada},                    // segundo disco interno
	}
	// ...
}
func TestNaDuvidaEPendrive(t *testing.T) { /* lerVolume falso com erro -> Pendrive */ }
func TestGuardaIdaEVolta(t *testing.T) { /* Gravar com colagem suja, Ler devolve a normalizada; arquivo 0600 fora do Windows */ }
func TestGuardaVaziaQuandoNaoExiste(t *testing.T) { /* Ler de pasta vazia == "" */ }
```

- [ ] **Passo 2: Rodar e ver falhar.** Run: `cd instalacao && go test ./guarda/... ./origem/...`. Expected: FAIL.
- [ ] **Passo 3: Implementar.** `lerVolume` fica atrás de uma variável de pacote para o teste trocar.
- [ ] **Passo 4: Rodar e ver passar**, com vet nos três GOOS. Expected: PASS.
- [ ] **Passo 5: Commit** `git commit -m "Instalação: guarda da chave e regra de pen drive ou disco"`

---

### Task 5: `instalacao`: copiar do pen drive com conferência

**Files:**
- Create: `instalacao/copiar/copiar.go`, `instalacao/copiar/copiar_test.go`

**Interfaces:**
- Produces:
  - `type Item struct { Origem, Destino string; Tamanho int64; SHA256 string }`. `SHA256` vem do `CHECKSUMS.sha256` e fica vazio para arquivos que não estão lá (`web/`, `core/`).
  - `copiar.Planejar(raiz, goos, goarch string) ([]Item, int64, error)`. Inclui `VERSION`, `LICENSE`, `web/`, `core/`, `bin/llamafile`, `models/*.gguf`, `bin/toolbox/<plat>/` e `bin/ctl/<plat>/`, além do lançador da plataforma: `IA de Bolso.exe` no Windows, `IA de Bolso.app/` no Mac. Fica de fora o resto. `<plat>` segue `ctl.Caminho` do lançador: no Windows é sempre `windows-amd64`; no Mac, `darwin-arm64` ou `darwin-amd64`.
  - `copiar.Copiar(ctx context.Context, itens []Item, destino string, prog func(feitos, total int64)) error`. Copia para `destino + ".novo"` e confere o SHA-256 de quem tem. Só no fim troca: `destino` vira `destino + ".velho"`, `.novo` vira `destino` e `.velho` é apagado. Em erro, apaga o `.novo` e deixa o `destino` intacto. No Mac, preserva o bit de execução do `.app` e não copia atributos estendidos.
  - `copiar.EspacoLivre(dir string) (int64, error)` e `var ErrSemEspaco`. `Copiar` exige total + 10% livres antes de começar.

- [ ] **Passo 1: Escrever os testes que falham.** Montam um pen drive falso em `t.TempDir()` com `CHECKSUMS.sha256` no formato `"<sha>  <caminho>"`:

```go
func TestPlanejarSoAPlataforma(t *testing.T) { /* com bin/toolbox/windows-amd64 e darwin-arm64: no plano de darwin/arm64 só aparece darwin-arm64 */ }
func TestCopiaConfereChecksum(t *testing.T) { /* modelo adulterado depois do Planejar -> erro e destino inexistente */ }
func TestCopiaQueFalhaNaoEstragaAnterior(t *testing.T) { /* destino com VERSION "0.14.0"; ctx cancelado no meio -> destino/VERSION ainda "0.14.0" e sem .novo */ }
func TestCopiaSubstituiAnterior(t *testing.T) { /* reinstalar por cima troca VERSION e não sobra .velho */ }
func TestProgressoChegaAoTotal(t *testing.T) { /* último prog(feitos,total) tem feitos == total */ }
```

- [ ] **Passo 2: Rodar e ver falhar.** Expected: FAIL.
- [ ] **Passo 3: Implementar.**
- [ ] **Passo 4: Rodar e ver passar.** Expected: PASS.
- [ ] **Passo 5: Commit** `git commit -m "Instalação: cópia do pen drive com conferência e troca atômica"`

---

### Task 6: `instalacao`: atalhos, início automático, desinstalar e o orquestrador

**Files:**
- Create: `instalacao/sistema/sistema.go`, `instalacao/sistema/windows.go`, `instalacao/sistema/darwin.go`, `instalacao/sistema/outros.go`, `instalacao/sistema/sistema_test.go`
- Create: `instalacao/instalar.go`, `instalacao/instalar_test.go`

**Interfaces:**
- Consumes: `copiar.*` (Task 5), `guarda.Valida` (Task 4).
- Produces:
  - `sistema.Destino(goos string, env func(string) string, home string) string`: os destinos das Global Constraints.
  - `sistema.Lancador(destino, goos string) string`: o caminho do executável instalado.
  - Interface `type Sistema interface { Atalhos(lancador string, areaDeTrabalho bool) error; LigarInicio(lancador string, ligar bool) error; InicioLigado() bool; Registrar(lancador, versao string) error; Remover() error; PlanoRemocao(destino string, pidLancador int) Comando }` e `sistema.Do(goos string) Sistema`.
  - `type Comando struct { Exe string; Args []string }`: o que o lançador dispara antes de sair para apagar a pasta.
  - `instalacao.Opcoes{ Raiz, Cache, Versao string; AreaDeTrabalho, Inicio bool }`.
  - `instalacao.Instalar(ctx, o Opcoes, prog func(feitos, total int64)) error`. Exige `guarda.Valida(o.Cache)`; se falhar, devolve `ErrSemChave`. Planeja, copia, cria os atalhos, liga ou desliga o início e registra.
  - `instalacao.Desinstalar(cache string, pidLancador int) (Comando, error)`. Desliga o início, remove os atalhos e o registro, apaga `<cache>/db.env` e devolve o `PlanoRemocao` da pasta. **Não apaga** `instalacao.chave`.

Por sistema:

- **Windows**
  - **Atalhos:** `.lnk` no Menu Iniciar (`%APPDATA%\Microsoft\Windows\Start Menu\Programs\CD2 IA de Bolso.lnk`) e na Área de Trabalho, por `powershell -NoProfile -WindowStyle Hidden` com `WScript.Shell`.
  - **Início:** `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, valor `CD2 IA de Bolso` = `"<lancador>" --inicio`.
  - **Registro:** `HKCU\...\Uninstall\CD2IADeBolso`, com `DisplayName "CD2 IA de Bolso"`, `DisplayVersion`, `Publisher "CD2 Tech"`, `UninstallString "<lancador>" --desinstalar` e `NoModify=1`.
  - **`PlanoRemocao`:** `cmd /c` que espera o PID sair (`tasklist` em laço, até 30 s) e roda `rmdir /s /q "<destino>"`.
- **Mac**
  - **Atalho na área de trabalho:** apelido por `osascript -e 'tell application "Finder" to make alias file ...'`.
  - **Início:** `~/Library/LaunchAgents/br.com.cd2.iadebolso.plist`, com `ProgramArguments [<executável dentro do .app>, "--inicio"]` e `RunAtLoad true`.
  - **Registro:** nada, porque Aplicativos já mostra o `.app`. Se o passo 3 da Task 0 pedir, cria o link simbólico em `~/Applications`.
  - **`PlanoRemocao`:** `sh -c` que espera o PID e roda `rm -rf`.

- [ ] **Passo 1: Escrever os testes que falham.** Os efeitos de sistema ficam atrás de funções de pacote trocáveis (`rodar`, `escreverRegistro`), e o teste confere os comandos e valores montados:

```go
func TestDestinos(t *testing.T) { /* windows: LOCALAPPDATA\Programs\CD2 IA de Bolso; darwin: home/Applications/IA de Bolso */ }
func TestInicioWindowsUsaRunDoUsuario(t *testing.T) { /* chave HKCU\...\Run, valor com --inicio entre aspas certas */ }
func TestLaunchAgentMac(t *testing.T) { /* plist gerado contém RunAtLoad true e --inicio */ }
func TestPlanoDeRemocaoWindowsAgendaDepoisDeSair(t *testing.T) { /* Comando espera o PID antes do rmdir */ }
func TestInstalarSemChaveRecusa(t *testing.T) { /* cache sem chave -> ErrSemChave e nada copiado */ }
func TestDesinstalarGuardaAChaveEApagaDbEnv(t *testing.T) { /* instalacao.chave continua; db.env some */ }
```

- [ ] **Passo 2: Rodar e ver falhar.** Expected: FAIL.
- [ ] **Passo 3: Implementar.** No `go.mod` do `instalacao`, acrescentar `golang.org/x/sys` (mesma versão do lançador, `v0.30.0`).
- [ ] **Passo 4: Rodar e ver passar**, com vet nos três GOOS. Expected: PASS.
- [ ] **Passo 5: Conferência manual no Mac** (este computador): um `go run` descartável instala de uma pasta de pen drive falsa em `$TMPDIR`, com `Destino` trocado. Confira o plist e o apelido, desinstale e veja que nada sobrou. No Windows, o Gabriel confere na Task 14.
- [ ] **Passo 6: Commit** `git commit -m "Instalação: atalhos, início automático, desinstalar"`

---

### Task 7: `cd2-ctl`: rotas da instalação

**Files:**
- Create: `ctl/internal/api/instalacao.go`, `ctl/internal/api/instalacao_test.go`, `ctl/internal/inst/inst.go`, `ctl/internal/inst/inst_test.go`
- Modify: `ctl/go.mod` (`require github.com/cd2tec/cd2-ia-de-bolso/instalacao v0.0.0` e `replace ... => ../instalacao`), `ctl/internal/api/api.go` (`Options.Instalacao`, o roteamento e `Allow-Methods`), `ctl/main.go`

**Interfaces:**
- Consumes: `origem.Detectar`, `codigo.Ler`, `guarda.*`, `instalacao.Instalar`, `sistema.Destino`, `sistema.Lancador` (Tasks 1 a 6).
- Produces, todas atrás das conferências de hoje (Host, Origin e `Authorization`):
  - `GET /instalacao` → `{"origem":"pendrive"|"instalada"|"desconhecida","codigo":"XXXX-XXXX-XXXX","temChave":bool,"instalada":bool}`. `instalada` diz se já existe uma instalação no destino.
  - `POST /instalacao/chave` `{"chave": "..."}` → 200 `{"ok":true}` se valer para este computador (e grava); 400 `{"erro":"Esta chave não é deste computador."}` se não valer.
  - `POST /instalacao` `{"areaDeTrabalho":bool,"inicio":bool}` → 202 e começa em segundo plano. 409 se já houver uma rodando. 403 `{"erro":"Falta a chave deste computador."}` sem chave válida.
  - `GET /instalacao/progresso` → `{"fase":"parada"|"copiando"|"pronta"|"erro","feitos":n,"total":n,"erro":"..."}`. Os textos de erro são em português: "Falta espaço no computador. Libere uns 3 GB e tente de novo." e "Não consegui copiar direito. Tente de novo."
  - `POST /instalacao/abrir` → roda `sistema.Lancador(destino)` com `--assumir`, sem esperar.
- `inst.Backend`: guarda o progresso com mutex. A raiz e o cache vêm do `--root` e do `--cache` que o `cd2-ctl` já recebe.

- [ ] **Passo 1: Escrever os testes que falham** no estilo de `api_test.go`, com um backend falso:

```go
func TestInstalacaoExigeChaveDaSessao(t *testing.T) { /* GET /instalacao sem Authorization -> 401 */ }
func TestInstalacaoStatus(t *testing.T) { /* 200 com origem e codigo do falso */ }
func TestChaveErradaDa400(t *testing.T) { /* corpo do erro é "Esta chave não é deste computador." */ }
func TestInstalarDuasVezesDa409(t *testing.T) {}
func TestSemBackendDa404(t *testing.T) { /* Options.Instalacao nil -> /instalacao 404, /status segue igual */ }
```

e, em `inst_test.go`, `TestProgressoVaiDeCopiandoAPronta` com `Instalar` falso.
- [ ] **Passo 2: Rodar e ver falhar.** Run: `cd ctl && go test ./...`. Expected: FAIL.
- [ ] **Passo 3: Implementar.** No `main.go`, monte o `inst.Backend` com `*root` e `*cache` e passe em `api.Options.Instalacao`.
- [ ] **Passo 4: Rodar e ver passar**, com `GOOS=windows go vet ./...`. Expected: PASS. Confira que `ctl-release.yml` compila com o `replace`: `cd ctl && CGO_ENABLED=0 GOOS=windows go build -o /dev/null .`.
- [ ] **Passo 5: Commit** `git commit -m "cd2-ctl: rotas da instalação"`

---

### Task 8: Web: lógica do modal (sem DOM)

**Files:**
- Create: `web/instalar.js` (padrão UMD dos outros: `module.exports` no node, `root.CD2Instalar` no navegador), `tests/instalar.test.js`

**Interfaces:**
- Consumes: as rotas da Task 7 e `/comprar` da Task 3.
- Produces:
  - `WORKER = 'https://instalariadebolso.cd2.com.br'`.
  - `urlComprar(codigo, porta): string`.
  - `normalizar(chave): string|null`: mesma regra da Task 1, conferida contra o `vetor.json`.
  - `telaInicial(status): 'compra'|'reinstalar'|null`. Devolve `null` se `status.origem !== 'pendrive'`, `'reinstalar'` se `temChave` e `'compra'` no resto.
  - `lerParametros(search): {abrir: boolean, chave: string|null}`, de `?instalar=1` e `?chave=`.
  - `entregarChave({canal, chave, esperarMs}): Promise<'entregue'|'sem-aba'>`. Manda pelo `BroadcastChannel('cd2-instalacao')` e espera `{tipo:'recebi'}` por até `esperarMs` (padrão 1500).
  - `ouvirChave({canal, aoReceber})`: responde `recebi` e chama `aoReceber(chave)`.
  - `textoProgresso(p): string`, por exemplo `copiando ... 1,4 de 2,4 GB`, com vírgula decimal e GB com uma casa.

- [ ] **Passo 1: Escrever os testes que falham**

```js
test('urlComprar', () => assert.equal(urlComprar('7F3K-Q9WD-2MXA', 8080),
  'https://instalariadebolso.cd2.com.br/comprar?codigo=7F3K-Q9WD-2MXA&porta=8080'));
test('telaInicial', () => {
  assert.equal(telaInicial({ origem: 'instalada' }), null);
  assert.equal(telaInicial({ origem: 'desconhecida' }), null);
  assert.equal(telaInicial({ origem: 'pendrive', temChave: true }), 'reinstalar');
  assert.equal(telaInicial({ origem: 'pendrive', temChave: false }), 'compra');
});
test('normalizar igual ao Go', () => { /* vetor.json com sujeira -> v.chave */ });
test('voltaSemAbaOriginal', async () => { /* canal falso sem resposta -> 'sem-aba' */ });
test('voltaComAbaOriginal', async () => { /* ouvirChave no mesmo canal falso -> 'entregue' e aoReceber(v.chave) */ });
test('textoProgresso', () => assert.equal(textoProgresso({ feitos: 1.4e9, total: 2.4e9 }), 'copiando ... 1,4 de 2,4 GB'));
```

- [ ] **Passo 2: Rodar e ver falhar.** Run: `node --test tests/instalar.test.js`. Expected: FAIL.
- [ ] **Passo 3: Implementar.**
- [ ] **Passo 4: Rodar e ver passar.** Run: `node --test`. Expected: PASS em tudo.
- [ ] **Passo 5: Commit** `git commit -m "Web: lógica do modal de instalação"`

---

### Task 9: Web: etiqueta, botão e modal na conversa

**Files:**
- Modify: `web/index.html` (CSS com os tokens do `:root`, `.topbar .tags` e `.actions`, `<dialog id="instalar">`, `<script src="instalar.js?v=...">`, ligação no `boot()`)
- Test: `tests/instalar.test.js` (o que for lógica), mais a conferência no navegador

**Interfaces:**
- Consumes: `CD2Instalar` (Task 8), `st.ctlKey` e o cliente do `cd2-ctl` que o `index.html` já tem (linhas 911 a 935).

O que fazer, com os textos aprovados no spec (seções 1 a 4):

- **Barra superior:** no `boot()`, depois de o `cd2-ctl` responder, `GET /instalacao`. Se `telaInicial` não for `null`, mostra a etiqueta amarela "rodando do pen drive" (StatusTag de Atenção, com as cores `--status-atencao` já usadas na faixa do sono) e o botão de contorno azul "Instalar no computador" antes de "Nova conversa". Sem `cd2-ctl`, nada aparece.
- **`<dialog id="instalar">`** com as telas abaixo, todas com o sobretítulo "Serviço de instalação digital" e a régua:
  - **compra:** o texto inteiro do spec, o código do computador e a caixa "Entendi...". O botão "Ir para o pagamento" abre `urlComprar` numa aba nova e passa para **aguardando**. Também tem o link "Já tenho a chave".
  - **aguardando:** "Aguardando o pagamento..." e o campo para colar a chave.
  - **reinstalar:** "Este computador já tem a chave", com o botão "Reinstalar com a minha chave", que vai para **perguntas**.
  - **perguntas:** as duas caixas, já marcadas, com os textos do spec, e o botão "Instalar".
  - **instalando:** a barra, o `textoProgresso` e "Não tire o pen drive agora". Pergunta o progresso a cada 500 ms.
  - **pronto:** os textos do spec, a chave, "Copiar a chave" e "Abrir a IA instalada", que chama `POST /instalacao/abrir`.
  - **erro:** a frase do erro e "Tentar de novo".
- **Colar a chave:** `POST /instalacao/chave`. Em 200, vai para **perguntas**; em 400, mostra o erro embaixo do campo.
- **Ao abrir a página:**
  - `?instalar=1` abre o modal.
  - `?chave=` em aba **sem** `ctlKey` roda `entregarChave`. Se a outra aba recebeu, mostra "Pode voltar para a outra janela da IA." Se não, mostra a chave e "Abra a IA de Bolso pelo pen drive e cole esta chave em Instalar no computador > Já tenho a chave."
  - `?chave=` em aba **com** `ctlKey` cola direto.
  - Depois disso, limpa o parâmetro do endereço, como já é feito com o `#k=`.
- **A aba com `ctlKey`** sempre roda `ouvirChave`, e receber a chave equivale a colar.

- [ ] **Passo 1:** Escrever no `tests/instalar.test.js` o teste que falha para a única lógica nova que sobrar fora do DOM. Se tudo já estiver no `instalar.js`, siga para o passo 2.
- [ ] **Passo 2: Implementar no `index.html`**, seguindo os componentes de hoje: o `dialog` de Configurações, as pílulas e a StatusTag.
- [ ] **Passo 3: Conferir no navegador** com um pen drive de desenvolvimento: `sh ctl/build.sh` e o lançador ou o `INICIAR-CD2-Mac.command` da pasta. Abra o modal, vá até o Stripe de teste, pague, volte e instale numa pasta trocada para o teste. Confira o console sem erros. Tire capturas da tela de compra e da tela pronta.
- [ ] **Passo 4: Rodar** `node --test`. Expected: PASS.
- [ ] **Passo 5: Commit** `git commit -m "Web: convite e modal de instalação na conversa"`

---

### Task 10: Lançador: a trava da cópia sem chave

**Files:**
- Create: `launcher/trava.go`, `launcher/trava_test.go`
- Modify: `launcher/go.mod` (`require` e `replace ../instalacao`), `launcher/janela.go` (`iniciar`: decide antes de `Rodar`), `launcher/frontend/dist/tela.js` (fase `Trancada`), `tests/lancador-tela.test.js`

**Interfaces:**
- Consumes: `origem.Detectar`, `guarda.Valida`, `guarda.Gravar`, `chave.Normalizar` (Tasks 1 e 4).
- Produces:
  - `deveTrancar(o origem.Origem, valida bool) bool`, que vale `o == Instalada && !valida`.
  - O método Wails `(a *App) ColarChave(chave string) string`: `""` se aceitou (grava e chama `go a.s.Rodar`), ou o texto do erro.
  - O método Wails `(a *App) ComoInstalar()`, que mostra o texto de como instalar.
  - O estado de tela `{fase:"Trancada"}`.
- Em `tela.js`, `render({fase:'Trancada'})` devolve:
  - título "Esta cópia não foi instalada neste computador";
  - linha "Para me usar daqui, instale pelo pen drive. Se você já pagou por este computador, cole a sua chave.";
  - botões `['Instalar neste computador', 'Já tenho a chave']`;
  - cena com o cérebro triste (`svgCerebro('sono', 0)`).
- "Já tenho a chave" troca para um campo de texto com "Liberar" (`ColarChave`) e "Voltar". "Instalar neste computador" mostra "Espete o pen drive da IA de Bolso e abra o lançador de lá. A instalação é feita por ele."

- [ ] **Passo 1: Escrever os testes que falham:** `TestDeveTrancar` (com a tabela das três origens) e `TestColarChaveDeOutroComputador`, que devolve "Esta chave não é deste computador.". Em `tests/lancador-tela.test.js`, adicionar `render Trancada` com o título e os botões exatos.
- [ ] **Passo 2: Rodar e ver falhar.** Run: `cd launcher && go test ./... ; node --test tests/lancador-tela.test.js`. Expected: FAIL.
- [ ] **Passo 3: Implementar.** No `iniciar`, se `deveTrancar`, emite o estado `Trancada`, não chama `Rodar` e deixa o ícone da bandeja só com "Mostrar" e "Fechar".
- [ ] **Passo 4: Rodar e ver passar**, com `GOOS=windows go vet .`. Expected: PASS.
- [ ] **Passo 5: Commit** `git commit -m "Lançador: trava da cópia sem chave"`

---

### Task 11: Lançador: convite, troca para a instalada e início escondido

**Files:**
- Create: `launcher/instalada.go`, `launcher/instalada_test.go`
- Modify: `launcher/main.go` (`SingleInstanceLock.OnSecondInstanceLaunch` lê os `Args`; `StartHidden` com `--inicio`), `launcher/janela.go`, `launcher/bandeja.go`, `launcher/internal/sessao/sessao.go` (`Deps.SemNavegador bool`: com ele, `carregar` não chama `AbrirNavegador`), `launcher/frontend/dist/tela.js`, `launcher/frontend/dist/estilo.css` (`.convite`, a caixinha tracejada), `tests/lancador-tela.test.js`

**Interfaces:**
- Consumes: `origem.Detectar` (Task 4), `POST /instalacao/abrir` (Task 7), que roda o lançador instalado com `--assumir`.
- Produces:
  - `type Args struct { Inicio, Assumir, Desinstalar bool }` e `lerArgs([]string) Args`.
  - O método Wails `(a *App) AbrirInstalar() error`, que abre `URL + "&instalar=1"`. A URL leva `?v=...#k=...`, então o parâmetro entra antes do `#`.
  - `render(estado, modo)` recebe `estado.pendrive: true` e, em `Pronta`, devolve `convite: {pergunta: 'Quer me usar sem o pen drive?', acao: 'Instale neste computador'}`.
  - O item da bandeja `itemInstalar = "Instalar neste computador"`, só com `pendrive` e `Pronta`.
  - No segundo lançamento com `--assumir`, o lançador que está rodando (o do pen drive) chama `a.Dormir()` e, depois do `Dormiu`, roda o executável de `SecondInstanceData.Args[0]` sem `--assumir` e sai.

- [ ] **Passo 1: Escrever os testes que falham:**
  - `TestLerArgs`;
  - `TestURLInstalarAntesDoHash`: `http://127.0.0.1:8080/?v=0.15.0&instalar=1#k=abc`;
  - `TestItensBandejaPendrive`, que confere que o item existe só em `Pronta` e só no pen drive;
  - `TestSemNavegadorNaoAbre`, em `sessao_test.go`;
  - `render Pronta com convite` e `render Pronta instalada sem convite`, em `tests/lancador-tela.test.js`.
- [ ] **Passo 2: Rodar e ver falhar.** Expected: FAIL.
- [ ] **Passo 3: Implementar.** Com `--inicio`, o app sobe com `StartHidden: true` e `Deps.SemNavegador: true`. Clicar no ícone continua abrindo a janelinha. "Abrir uma nova janela da conversa" funciona normalmente, porque a URL existe em `Pronta`.
- [ ] **Passo 4: Rodar e ver passar**, com vet nos três GOOS. Expected: PASS.
- [ ] **Passo 5: Commit** `git commit -m "Lançador: convite para instalar, troca para a instalada e início escondido"`

---

### Task 12: Lançador instalado: acordar com o computador e desinstalar

**Files:**
- Modify: `launcher/bandeja.go`, `launcher/instalada.go`, `launcher/janela.go`, `launcher/frontend/dist/tela.js`, `launcher/frontend/dist/piadas.js` (se precisar), `tests/lancador-tela.test.js`
- Test: `launcher/instalada_test.go`

**Interfaces:**
- Consumes: `sistema.Do(goos)` (`InicioLigado`, `LigarInicio`) e `instalacao.Desinstalar` (Task 6).
- Produces:
  - Itens da bandeja na instalada: `itemInicio = "Acordar quando o computador ligar"`, com marcação, e `itemDesinstalar = "Desinstalar..."`, na ordem do rascunho (`telas-instalacao.html`, tela 4).
  - Os métodos Wails `(a *App) PedirDesinstalar()`, que mostra a tela, e `(a *App) Desinstalar()`. `Desinstalar` faz, nesta ordem:
    1. `Dormir`;
    2. espera o `Dormiu`;
    3. chama `instalacao.Desinstalar`;
    4. dispara o `Comando`, desligado do processo;
    5. sai.
  - `render({fase:'Desinstalar'})` devolve:
    - título "Quer mesmo me desinstalar?";
    - linha "Apago a IA deste computador, os atalhos e o acordar com o computador. A sua conexão com o banco também sai. Para voltar, use a chave que foi para o seu e-mail.";
    - botões `['Desinstalar', 'Melhor não']`.
  - Com `--desinstalar` (vindo de Configurações > Aplicativos), o lançador abre direto nessa tela, sem acordar a IA. Se outra cópia estiver rodando, o segundo lançamento a manda para essa tela.

- [ ] **Passo 1: Escrever os testes que falham:**
  - `TestItensBandejaInstalada`, que confere a ordem e a marcação conforme `InicioLigado`;
  - `TestDesinstalarDormeAntes`, com `sistema` e `sessao` falsos que registram a ordem: dormiu, depois desinstalou, depois disparou;
  - `render Desinstalar`, em `tests/lancador-tela.test.js`.
- [ ] **Passo 2: Rodar e ver falhar.** Expected: FAIL.
- [ ] **Passo 3: Implementar.** A marcação do systray usa `MenuItem.Check()`/`Uncheck()`, que já existem no `fyne.io/systray`.
- [ ] **Passo 4: Rodar e ver passar**, com vet nos três GOOS. Expected: PASS.
- [ ] **Passo 5: Commit** `git commit -m "Lançador instalado: acordar com o computador e desinstalar"`

---

### Task 13: Condições do serviço, documentação, versão e release

**Files:**
- Create: `docs/condicoes-instalacao.html` (mesma identidade do `docs/index.html`; o texto vem do "Como funciona" do modal, mais a frase sobre reembolso: "Em caso de reembolso, a instalação já feita continua funcionando.")
- Modify: `README.md` (seção nova "Instalar no computador", depois de "2. Usar"), `docs/criar-pendrive.md` (como atualizar uma instalada: pôr para dormir e abrir pelo pen drive; avisos do sistema anotados na Task 0, passo 2), `VERSION`, os `?v=` do `web/index.html`, a versão fixada do `cd2-ctl` em `core/prepare-unix.sh`, `core/prepare-windows.ps1` e no manifesto do instalador (`installer/internal/manifesto`)

- [ ] **Passo 1:** Escrever o teste que falha em `tests/versao.test.js`, se ele ainda não cobrir: os `?v=` do `index.html` iguais ao `VERSION`, incluindo `instalar.js`.
- [ ] **Passo 2:** Subir o `VERSION` e os `?v=`. Rodar `node --test`. Expected: PASS.
- [ ] **Passo 3:** Escrever as condições, o README e o `criar-pendrive.md`, nas regras de texto do projeto.
- [ ] **Passo 4:** Fixar a versão nova do `cd2-ctl` nos scripts de preparo e no manifesto. Rodar `cd installer && go test ./...`. Expected: PASS.
- [ ] **Passo 5: Commit** `git commit -m "Instalação paga: condições, documentação e versão 0.15.0"`

---

### Task 14: Conferência final em máquinas reais (manual, com o Gabriel)

Grave um pen drive com a branch, com o Worker publicado no modo de teste. Em um Windows 11 e em um Mac Apple Silicon, faça:

- [ ] **Pen drive:** a etiqueta e o botão aparecem na barra, a caixinha aparece no lançador e o item aparece no menu.
- [ ] **Compra:** pagar com `4242 4242 4242 4242`, voltar e instalar com as duas caixas marcadas. Conferir o atalho, o item nos programas e o e-mail de teste com a chave.
- [ ] **Troca:** "Abrir a IA instalada" põe a do pen drive para dormir e abre a instalada. Tirar o pen drive e conferir que a conversa responde.
- [ ] **Reinício:** reiniciar o computador. A IA acorda escondida no ícone, sem abrir janela.
- [ ] **Acordar com o computador:** desmarcar no menu e reiniciar. Ela não acorda.
- [ ] **Cópia para outro computador:** copiar a pasta instalada para outro computador e abrir. Aparece "Esta cópia não foi instalada neste computador". Colar a chave do primeiro computador mostra "Esta chave não é deste computador.".
- [ ] **Desinstalar pelo menu:** a pasta, os atalhos e o início somem, e `instalacao.chave` continua no cache. No Windows, desinstalar também por Configurações > Aplicativos.
- [ ] **Reinstalar pelo pen drive:** o modal oferece "Reinstalar com a minha chave", sem pagamento.
- [ ] **Volta com a aba original fechada:** pagar, fechar a conversa antes de voltar, abrir de novo pelo lançador e colar a chave da página do Worker. A instalação segue.
