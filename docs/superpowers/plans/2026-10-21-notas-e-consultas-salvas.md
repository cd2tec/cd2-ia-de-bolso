# Notas da TI e consultas salvas — plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A TI escreve notas por tabela e por campo que entram no prompt; resultados aprovados com 👍 viram consultas salvas, com parâmetros, que rodam sem o modelo; o trabalho é exportado (pen drive e instalada) e importado (só instalada).

**Architecture:** O cd2-ctl ganha um pacote `notas` que guarda um arquivo JSON por banco na pasta de cache (a mesma do `db.env`, que o pen drive e a instalada já compartilham no mesmo computador) e as rotas `GET/POST /notas` e `POST /notas/importar` (esta só quando a IA roda instalada). Toda a lógica (aplicar notas no catálogo, teto no prompt, detectar literais, montar o SQL com parâmetros, plano de importação) fica em módulos JS puros e testáveis (`web/notas.js`, `web/consultas.js`); o `index.html` só faz DOM e chamadas.

**Tech Stack:** Go 1.23 (cd2-ctl, `net/http`, testes `go test`), JavaScript sem build (módulos no padrão `(function(root){...})` com `module.exports` no Node), testes `node --test`.

**Spec:** [docs/superpowers/specs/2026-10-21-notas-e-consultas-salvas-design.md](../specs/2026-10-21-notas-e-consultas-salvas-design.md)

## Global Constraints

- Nota de tabela: até **300** caracteres. Nota de campo: até **120**. Teto de notas por pergunta: **2000** caracteres.
- Ordem de corte acima do teto: primeiro notas de campo das tabelas de menor pontuação, depois a descrição da IA das tabelas que têm nota da TI; a nota da TI sobre a tabela nunca é cortada.
- Coluna com nota sempre entra entre as colunas da tabela escolhida.
- Importar: junta; só o item preenchido dos dois lados com texto diferente é conflito e é sobrescrito pelo arquivo, com aviso e confirmação. Nota vazia não é exportada. Consulta com o mesmo nome conta como conflito.
- Importar só quando a origem é `instalada` (`instalacao/origem`); no pen drive o botão não aparece e a rota responde 403.
- Banco identificado por motor, servidor (minúsculas), porta, base e schema efetivo (PostgreSQL vazio→`public`, SQL Server vazio→`dbo`, Oracle vazio→usuário em maiúsculas, MariaDB→base, Firebird/DBF→vazio). Sem senha em arquivo nenhum.
- Consulta salva roda pelo mesmo `db_read_only_query`, com `guardSql` e `prepareSql`, sem chamar o modelo.
- Nada de `localStorage` para notas ou consultas.
- Botão "Entendimento do banco" e barra lateral escondidos no modo Demonstração e sem banco conectado.
- Textos de tela em português do Brasil, iguais aos esboços aprovados (`.superpowers/brainstorm/39594-1792614092/content/telas-v1.html`, `parametros-v2.html`, `exportar-importar-v1.html`).
- Toda versão anda junto: `VERSION`, `?v=` dos scripts no `index.html` e `version:` do `mcp.js` (vai para **0.17.0** na última tarefa).

## Review Focus

1. **Tabela ou campo apagado/renomeado no banco depois da nota:** a nota continua no arquivo, não aparece na tela nem no prompt, e nada quebra (teste na Tarefa 3).
2. **Valor de parâmetro com aspas ou `%`** (ex.: `D'ÁVILA`): vai escapado (`''`) e nunca fecha a string (teste na Tarefa 5).
3. **Números dentro de identificadores** (`vendas_2023`, `t1.col`, `"Loja 3"`) e texto dentro de comentário não viram literais (teste na Tarefa 5).
4. **Arquivo de importação estragado, de outro formato ou grande demais:** erro claro e nada é gravado (testes nas Tarefas 2 e 3).
5. **ctl fora do ar ou banco trocado com a tela de notas aberta:** salvar mostra erro e não apaga o texto digitado; trocar de banco fecha a tela (verificação manual na Tarefa 6).

---

### Task 1: Pacote `notas` no cd2-ctl (chave do banco e arquivo)

**Files:**
- Create: `ctl/internal/notas/notas.go`
- Test: `ctl/internal/notas/notas_test.go`

**Interfaces:**
- Produces:
  - `type Banco struct { Motor, Host string; Porta int; Base, Usuario, Schema string }` (JSON: `motor, host, porta, base, schema`; `Usuario` com `json:"-"`).
  - `func SchemaEfetivo(b Banco) string`
  - `func Chave(b Banco) string` — 32 hex (primeiros 16 bytes do SHA-256 de `motor|host minúsculo|porta|base|schemaEfetivo`).
  - `type Store struct` com `func New(dir string) *Store`, `func (s *Store) Ler(b Banco) (json.RawMessage, error)` (arquivo ausente → `{"versao":1,"tabelas":{},"consultas":[]}`), `func (s *Store) Gravar(b Banco, dados json.RawMessage) error`.
  - `const MaxDados = 1 << 20`; `var ErrDados = errors.New("notas: conteúdo inválido")`.

- [ ] **Step 1: Write the failing tests**

```go
func TestSchemaEfetivo(t *testing.T) {
	cases := []struct{ b Banco; want string }{
		{Banco{Motor: "postgres"}, "public"},
		{Banco{Motor: "postgres", Schema: "Erp"}, "Erp"},
		{Banco{Motor: "sqlserver"}, "dbo"},
		{Banco{Motor: "oracle", Usuario: "loja"}, "LOJA"},
		{Banco{Motor: "mariadb", Base: "erp"}, "erp"},
		{Banco{Motor: "firebird", Schema: "x"}, ""},
		{Banco{Motor: "dbf"}, ""},
	}
	// assert SchemaEfetivo(c.b) == c.want para cada caso
}
func TestChaveIgnoraCaixaDoHostESchemaPadrao(t *testing.T) {
	a := Chave(Banco{Motor: "postgres", Host: "Servidor", Porta: 5432, Base: "erp"})
	b := Chave(Banco{Motor: "postgres", Host: "servidor", Porta: 5432, Base: "erp", Schema: "public"})
	// a == b; len(a) == 32; Chave com Porta 5433 != a; Chave com Host "127.0.0.1" != Chave com "localhost"
}
func TestLerSemArquivoDevolveVazio(t *testing.T)            // Ler num dir vazio == `{"versao":1,"tabelas":{},"consultas":[]}`
func TestGravarELer(t *testing.T)                          // grava, relê igual; arquivo em dir/notas/<Chave>.json com modo 0600 (pular modo no Windows)
func TestGravarRecusaInvalido(t *testing.T)                 // "não é json", `[]`, `{"versao":2}` e 1 MiB+1 → errors.Is(err, ErrDados), arquivo anterior intacto
func TestGravarNaoTemSenha(t *testing.T)                    // Banco com Usuario "u" → o arquivo gravado não contém "u" fora dos dados
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ctl && go test ./internal/notas/`
Expected: FAIL (pacote não existe)

- [ ] **Step 3: Implement `notas.go`**

`Gravar` aceita só objeto JSON com `"versao":1`, `tabelas` objeto e `consultas` array, até `MaxDados`; escreve em `<arquivo>.tmp` (0600) e renomeia. Cria `dir/notas` com 0700. Comentários em português, no tom de `control.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ctl && gofmt -l . && go vet ./... && go test ./internal/notas/`
Expected: nenhum arquivo listado pelo gofmt; PASS

- [ ] **Step 5: Commit**

```bash
git add ctl/internal/notas
git commit -m "cd2-ctl: notas por banco na pasta de cache"
```

---

### Task 2: Rotas `/notas` e `/notas/importar`

**Files:**
- Modify: `ctl/internal/api/api.go` (Options, server, switch de rotas)
- Create: `ctl/internal/api/notas.go`
- Modify: `ctl/main.go:106-110` (ligar o Store e a origem)
- Test: `ctl/internal/api/notas_test.go`

**Interfaces:**
- Consumes: `notas.Banco`, `notas.Store`, `notas.ErrDados`, `notas.MaxDados` (Task 1); `control.Status` (campos `Estado, Motor, Host, Porta, Base, Usuario, Schema`).
- Produces (HTTP, mesmas checagens de Host/Origin/chave das outras rotas):
  - `GET /notas` → 200 `{"banco":{motor,host,porta,base,schema},"dados":{...}}`; 409 `{"erro":"Nenhum banco conectado."}` quando `Status().Motor == ""`.
  - `POST /notas` corpo `{"dados":{...}}` → 200 `{"ok":true}`; 400 com `notas.ErrDados`; 413 acima de `MaxDados`; 409 sem banco.
  - `POST /notas/importar` mesmo corpo → 403 `{"erro":"Importar só na versão instalada."}` quando `PodeImportar()` é falso; senão igual ao `POST /notas`.
  - `Options` ganha `Notas *notas.Store` e `PodeImportar func() bool`. Com `Notas == nil`, as rotas dão 404.

- [ ] **Step 1: Write the failing tests** (usar `do`/`erro` de `api_test.go`; um `fakeBackend` com `Status()` configurável)

```go
func TestNotasSemBanco(t *testing.T)        // GET /notas com Status{} → 409 "Nenhum banco conectado."
func TestNotasGravarELer(t *testing.T)      // POST /notas {"dados":{"versao":1,"tabelas":{"vendas":{"nota":"oficial"}},"consultas":[]}} → 200; GET devolve banco.motor "postgres" e a mesma nota
func TestNotasInvalido(t *testing.T)        // POST /notas {"dados":[]} → 400; corpo de 1 MiB+1 → 413
func TestImportarNoPendrive(t *testing.T)   // PodeImportar → false: POST /notas/importar → 403 "Importar só na versão instalada." e nada gravado
func TestImportarInstalada(t *testing.T)    // PodeImportar → true: 200 e GET devolve o importado
func TestNotasDesligadas(t *testing.T)      // Options sem Notas → GET /notas 404
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ctl && go test ./internal/api/`
Expected: FAIL (rotas inexistentes)

- [ ] **Step 3: Implement**

Em `notas.go` da API, ler o corpo com `http.MaxBytesReader(w, r.Body, notas.MaxDados+4096)` e `DisallowUnknownFields`; montar `notas.Banco` a partir de `s.b.Status()`. Em `main.go`: `Notas: notas.New(*cache)` e `PodeImportar: func() bool { return instalacao.Info().Origem == "instalada" }`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ctl && gofmt -l . && go vet ./... && go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add ctl
git commit -m "cd2-ctl: rotas de notas e importação só na instalada"
```

---

### Task 3: `web/notas.js` — aplicar, cortar, exportar e plano de importação

**Files:**
- Create: `web/notas.js` (global `CD2Notas`, sem DOM)
- Test: `tests/notas.test.js`

**Interfaces:**
- Consumes: `Cat.tokens` de `web/catalog.js`.
- Produces:
  - `LIMITE = { tabela:300, campo:120, prompt:2000 }`
  - `vazio()` → `{ versao:1, tabelas:{}, consultas:[] }`. Formato: `tabelas[nomeTabela] = { nota:string, campos:{ [nomeCampo]:string } }`; `consultas[] = { id, nome, sql, visual, x, y, params:[{ nome, tipo, inicio, fim, valor }], criada }`.
  - `limpar(texto, max)` → string sem controle, espaços colapsados, cortada em `max`.
  - `aplicar(catalog, dados)` → muta o catálogo: `t.nota`, `t.notaTokens` (via `Cat.tokens(nota, true)`), `c.nota` em cada coluna; remove notas antigas antes. Tabelas/campos que não existem no catálogo são ignorados.
  - `contar(dados)` → `{ tabelasComNota:number }`.
  - `cortar(tabelas, max)` → cópia de `tabelas` (ordem por pontuação, maior primeiro) com `columns[].nota` e `perfil.descricao` removidos conforme a ordem de corte, até a soma de `nota` + notas de campo + descrições de tabelas com nota caber em `max`.
  - `exportar(dados, banco, agora:Date)` → `{ nome:'ia-de-bolso-notas-<base>-AAAA-MM-DD.json', conteudo:string }` com `{ formato:'ia-de-bolso-notas', versao:1, exportado:ISO, banco, dados }`, sem notas vazias.
  - `lerArquivo(texto)` → `{ ok:true, banco, dados }` ou `{ ok:false, erro }` (`'Este arquivo não é uma exportação da IA de Bolso.'`).
  - `mesmoBanco(a, b)` → boolean (motor, host minúsculo, porta, base, schema).
  - `planoImportacao(atual, chegando)` → `{ novas:{ notas:number, consultas:number }, conflitos:[{ tipo:'tabela'|'campo'|'consulta', onde:string, antes:string, depois:string }], resultado:dados }`.

- [ ] **Step 1: Write the failing tests**

```js
test('aplicar põe nota na tabela e no campo e ignora o que sumiu do banco', () => {
  const cat = { tables:[{ name:'vendas', tokens:[], columns:[{ name:'status', tokens:[] }] }] };
  N.aplicar(cat, { versao:1, consultas:[], tabelas:{ vendas:{ nota:'Tabela oficial', campos:{ status:'9 = cancelado', antiga:'x' } }, apagada:{ nota:'y', campos:{} } } });
  assert.equal(cat.tables[0].nota, 'Tabela oficial');
  assert.equal(cat.tables[0].columns[0].nota, '9 = cancelado');
  assert.ok(cat.tables[0].notaTokens.includes('oficial'));
});
test('limpar corta em 300 e tira quebra de linha', ...);          // 'a\nb' → 'a b'; 400 x 'a' com max 300 → length 300
test('cortar: tira notas de campo da última tabela primeiro, nunca a nota da tabela', ...);
// duas tabelas com nota de 100, 30 notas de campo de 120 cada; cortar(t, 2000): t[0].nota e t[1].nota intactas, notas de campo de t[1] somem antes das de t[0], total <= 2000
test('cortar: depois das notas de campo, tira a descrição da IA de tabela com nota', ...);
test('exportar não leva nota vazia e dá nome com a base e a data', ...); // nome 'ia-de-bolso-notas-loja01-2026-10-21.json'
test('lerArquivo recusa JSON estragado e outro formato', ...);      // '{' e '{"formato":"x"}' → ok:false com a mensagem fixa
test('planoImportacao junta e só sobrescreve o que está preenchido dos dois lados', ...);
// atual: vendas.nota 'A', lojas.nota 'L', consulta 'Top' sql1; chegando: vendas.nota 'B', produtos.nota 'P', consulta 'top ' sql2, consulta 'Nova'
// → conflitos: tabela vendas (A→B) e consulta Top; novas.notas 1, novas.consultas 1; resultado mantém lojas 'L'
test('planoImportacao: mesmo texto dos dois lados não é conflito', ...);
test('mesmoBanco ignora caixa do host', ...);
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `node --test tests/notas.test.js`
Expected: FAIL (`Cannot find module '../web/notas.js'`)

- [ ] **Step 3: Implement `web/notas.js`** no padrão de `web/perfil.js` (cabeçalho em comentário, `"use strict"`, `isNode`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `node --test tests/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/notas.js tests/notas.test.js
git commit -m "Notas da TI: aplicar no catálogo, teto do prompt e plano de importação"
```

---

### Task 4: Notas na escolha das tabelas e no prompt

**Files:**
- Modify: `web/catalog.js:95-99` (`pickColumns`) e `:140-167` (`rankTables`)
- Modify: `web/ask.js:180-215` (`buildMessages`) e `:258-290` (`ask`)
- Test: `tests/catalog.test.js`, `tests/ask.test.js`

**Interfaces:**
- Consumes: `t.nota`, `t.notaTokens`, `c.nota` (Task 3 `aplicar`); `CD2Notas.cortar`, `CD2Notas.LIMITE.prompt`.
- Produces:
  - `rankTables` conta `notaTokens` como `descricaoTokens` (peso 1.5, máximo 3) e devolve `nota` em cada tabela; `pickColumns` devolve `nota` e sempre inclui toda coluna com nota (mesmo além de `n`).
  - Linha da tabela no prompt: `- <tabela> (<tamanho>; <papel>): nota da TI: <nota> | descrição: <desc> | colunas: <col> <tipo> (nota da TI: <nota do campo>), ...`.
  - `ask()` passa a devolver `sqlBase` (o SQL depois de `guardSql`, antes de `prepareSql`) no resultado ok.

- [ ] **Step 1: Write the failing tests**

```js
// catalog.test.js
test('rankTables: nota da TI faz a tabela entrar mesmo sem bater no nome', ...); // tabela 'tb_x' com nota 'cupons de venda oficiais' entra para 'vendas do mês'
test('pickColumns: coluna com nota entra mesmo além do limite de 25', ...);      // 40 colunas, a 39ª com nota → presente, com .nota
// ask.test.js
test('buildMessages: nota da tabela vem antes da descrição e nota do campo junto do campo', () => {
  // assert.match(sys, /- vendas \(cerca de 10 linhas; movimento[^)]*\): nota da TI: Tabela oficial \| descrição: cupons \| colunas: .*status int \(nota da TI: 9 = cancelado\)/)
});
test('buildMessages: notas acima de 2000 caracteres são cortadas pela ordem do spec', ...);
test('ask devolve sqlBase sem o limite do prepareSql', ...);
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `node --test tests/catalog.test.js tests/ask.test.js`
Expected: FAIL nos testes novos

- [ ] **Step 3: Implement** (em `ask.js`, `require('./notas.js')` no Node e `root.CD2Notas` no navegador, como `Cat`; `buildMessages` passa `o.tabelas` por `Notas.cortar(o.tabelas, Notas.LIMITE.prompt)` antes de montar as linhas).

- [ ] **Step 4: Run tests to verify they pass**

Run: `node --test tests/`
Expected: PASS (inclusive os testes antigos de `buildMessages`)

- [ ] **Step 5: Commit**

```bash
git add web/catalog.js web/ask.js tests/catalog.test.js tests/ask.test.js
git commit -m "Notas da TI entram na escolha das tabelas e no prompt"
```

---

### Task 5: `web/consultas.js` — literais, parâmetros e rodar sem o modelo

**Files:**
- Create: `web/consultas.js` (global `CD2Consultas`, sem DOM)
- Modify: `web/ask.js` (nova `rodarSalva`, exportada em `api`)
- Test: `tests/consultas.test.js`, `tests/ask.test.js`

**Interfaces:**
- Produces:
  - `acharLiterais(sql)` → `[{ valor, tipo:'data'|'numero'|'texto', inicio, fim, marcado:boolean }]`. `inicio/fim` são posições no `sql` só do conteúdo (dentro das aspas, sem os `%` das pontas de um LIKE). Data = `'AAAA-MM-DD'` ou `'AAAA-MM-DD HH:MM:SS'`, `marcado:true`; número e texto, `marcado:false`. Ignora identificadores (`vendas_2023`, `t1.col`, `"Loja 3"`, `[Loja 3]`, `` `x1` ``) e comentários (`--`, `/* */`).
  - `paramDeTrecho(sql, inicio, fim)` → `{ ok:true, param:{ valor, tipo, inicio, fim } }` ou `{ ok:false, erro:'Selecione um valor dentro de aspas ou um número inteiro.' }`.
  - `paramDeValor(sql, valor)` → primeira ocorrência válida, mesmo formato; ausente → `{ ok:false, erro:'Esse valor não aparece na consulta.' }`.
  - `montarSql(consulta, valores)` (`valores[i]` para `consulta.params[i]`, data em `AAAA-MM-DD`) → `{ ok:true, sql }` ou `{ ok:false, erro:'<nome>: data inválida.' | '<nome>: número inválido.' }`. Texto com `'` vira `''`. Substitui do último para o primeiro.
  - `ask.js`: `async rodarSalva({ consulta, valores, motor, runSql, signal })` → mesmo formato de `ask()` (ok com `titulo = consulta.nome`, `visual/x/y` de `pickVisual({ visual, x, y }, ...)`, `frase`, `colunas`, `linhas`, `todas`, `sql`, `valores`), ou `{ ok:false, motivo:'sql'|'vazio'|'timeout'|'timeout_front', sql }`, ou `{ ok:false, motivo:'param', erro }`.

- [ ] **Step 1: Write the failing tests**

```js
const SQL = "SELECT l.nome AS loja, SUM(v.total) AS total FROM vendas_2023 v JOIN lojas l ON l.cod = v.cod_loja "
  + "WHERE v.dt_mov >= '2026-10-01' AND v.dt_mov < '2026-11-01' AND v.status <> 9 AND UPPER(l.cidade) LIKE '%CURITIBA%' -- 77\nGROUP BY l.nome";
test('acharLiterais: datas marcadas, número e texto do LIKE sem os %', () => {
  assert.deepEqual(C.acharLiterais(SQL).map(l => [l.valor, l.tipo, l.marcado]),
    [['2026-10-01','data',true], ['2026-11-01','data',true], ['9','numero',false], ['CURITIBA','texto',false]]);
});
test('acharLiterais ignora número em identificador, aspas duplas, colchetes e comentário', ...); // "Loja 3", [Loja 3], t1.col, vendas_2023, -- 77
test('paramDeTrecho aceita trecho dentro de aspas e recusa pedaço de identificador', ...);
test('paramDeValor acha CURITIBA e recusa valor ausente', ...);
test("montarSql escapa aspas: D'ÁVILA vira D''ÁVILA dentro do LIKE", ...);
test('montarSql recusa data e número inválidos com o nome do parâmetro', ...); // '2026-02-30' → 'Data inicial: data inválida.'
// ask.test.js
test('rodarSalva roda sem chamar o modelo e usa o visual salvo', ...); // runSql falso recebe o SQL com os valores novos e o limite do prepareSql; resultado.titulo == consulta.nome
test('rodarSalva recusa SQL que não é leitura', ...);                  // sql 'DELETE FROM x' → motivo 'sql', runSql não chamado
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `node --test tests/consultas.test.js tests/ask.test.js`
Expected: FAIL

- [ ] **Step 3: Implement** (varredura única do SQL com estados: fora, string `'…'` com `''`, identificador `"…"`/`[…]`/`` `…` ``, comentário de linha e de bloco; número só quando o caractere antes não é letra, dígito, `_`, `$`, `#` ou `.`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `node --test tests/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/consultas.js web/ask.js tests/consultas.test.js tests/ask.test.js
git commit -m "Consultas salvas: parâmetros e execução sem o modelo"
```

---

### Task 6: Tela "Entendimento do banco"

**Files:**
- Modify: `web/index.html` (botão no topo, nova tela, carregar/salvar pelo ctl, `<script src="notas.js?v=0.16.0">` e `consultas.js` antes de `ask.js`)
- Modify: `web/report.js` (`perfilHtml` mostra `t.nota` e as notas de campo)
- Test: `tests/report.test.js`

**Interfaces:**
- Consumes: `GET/POST /notas` (Task 2) via `ctlRequest`; `CD2Notas.aplicar/limpar/contar/LIMITE` (Task 3).
- Produces (estado em `st`, usado pelas Tarefas 7 e 8): `st.notas` (dados do ctl), `st.notasBanco` (`banco` do ctl), `async carregarNotas()` (chamada quando o catálogo fica pronto, aplica no `st.catalog`), `async salvarNotas()` → `true|false` (POST, reaplica no catálogo).

- [ ] **Step 1: Write the failing test**

```js
test('perfilHtml mostra a nota da TI da tabela e dos campos', ...); // tabela com nota 'Tabela oficial' e coluna status com nota → HTML contém as duas, escapadas
```

- [ ] **Step 2: Run to verify it fails** — `node --test tests/report.test.js` → FAIL

- [ ] **Step 3: Implement**

- Botão `#bNotas` "Entendimento do banco" entre `#bNew` e `#bCfg`, só visível com banco conectado, catálogo pronto e fora do modo Demonstração.
- Tela em tela cheia (mesmo padrão de troca de tela do app), conforme a opção A de `telas-v1.html`: topo com "← Voltar para a conversa", título e resumo (`motor · base · N tabelas, M com nota`); lista à esquerda com busca por tabela ou campo, papel e "📝 tem nota"; detalhe à direita com "O que a IA entendeu" (só leitura; se a descrição ainda está na fila, "A IA ainda está lendo esta tabela."), "Nota da TI sobre a tabela" (`maxlength=300`, contador), grade de campos com nota (`maxlength=120`) e "Salvar notas" com "Salvo neste computador".
- Erro ao salvar: mensagem na tela, texto digitado continua nas caixas.
- Trocar de banco (`st.catalogGen` muda) com a tela aberta: fecha a tela.
- `perfilHtml` com as notas.

- [ ] **Step 4: Run tests** — `node --test tests/` → PASS

- [ ] **Step 5: Verificação no app**

Subir o banco de teste (`docker compose -p cd2eval -f tests/eval/docker-compose.yml up -d --wait`), abrir a IA (`sh ia-de-bolso.sh`), conectar em `127.0.0.1:55433` (erp/erp/erp), escrever uma nota de tabela e de campo, salvar, recarregar a página e conferir que voltaram; perguntar algo e conferir no prompt (log do motor) a linha `nota da TI:`. Parar o cd2-ctl e tentar salvar: aparece erro e o texto fica.

- [ ] **Step 6: Commit**

```bash
git add web/index.html web/report.js tests/report.test.js
git commit -m "Tela Entendimento do banco com notas da TI"
```

---

### Task 7: 👍 Salvar consulta, barra lateral e rodar de novo

**Files:**
- Modify: `web/index.html` (`renderAnswer`, barra lateral, janelas "Salvar consulta" e de parâmetros)

**Interfaces:**
- Consumes: `st.notas`, `salvarNotas()` (Task 6); `CD2Consultas.acharLiterais/paramDeTrecho/paramDeValor/montarSql` (Task 5); `CD2Ask.rodarSalva` e `r.sqlBase` (Tasks 4 e 5).

- [ ] **Step 1: Implement**

- `renderAnswer`: botão "👍 Salvar consulta" antes de CSV; depois de salvo, "👍 Salva" desabilitado. Resposta de consulta salva mostra abaixo do título os valores usados (`Data inicial: 01/10/2026 · Data final: 01/11/2026`).
- Janela "Salvar consulta" igual a `parametros-v2.html`: Nome (título do resultado), SQL com literais destacados, tabela de literais com caixa de marcar, "Perguntar como" e tipo; caixa "+ Adicionar parâmetro" (seleção do mouse no SQL via `selectionStart/selectionEnd` de um `<textarea readonly>` ou valor digitado), com o erro de `paramDeTrecho/paramDeValor` em tela. Sem literais, só o Nome. Parâmetro marcado sem nome não deixa salvar ("Dê um nome a cada valor marcado.").
- Grava `{ id:'c' + Date.now(), nome, sql:r.sqlBase, visual, x, y, params, criada }` em `st.notas.consultas` e chama `salvarNotas()`.
- Barra lateral "Consultas salvas (N)" à esquerda do chat, com busca por nome, rolagem, item com nome, visual e data; menu do item com Renomear e Apagar (confirmação). Escondida no modo Demonstração e sem banco.
- Clique: sem parâmetros roda direto; com parâmetros abre a janela com os valores da última vez e "Rodar". Erro de `montarSql` aparece na janela. Depois de rodar, grava os valores usados como `valor` dos parâmetros.
- Durante `st.busy`, os cliques da barra esperam (mesma regra do Enviar).

- [ ] **Step 2: Run tests** — `node --test tests/` → PASS

- [ ] **Step 3: Verificação no app**

No banco de teste: perguntar "vendas por loja em outubro de 2026", 👍, marcar as datas, salvar; recarregar e ver a consulta na barra; rodar com outras datas e conferir que o modelo não foi chamado (log do motor sem pedido novo); adicionar `CURITIBA` como parâmetro por seleção e rodar com `D'ÁVILA` sem erro de SQL; buscar, renomear e apagar.

- [ ] **Step 4: Commit**

```bash
git add web/index.html
git commit -m "Consultas salvas: joinha, barra lateral e parâmetros"
```

---

### Task 8: Exportar, importar e versão 0.17.0

**Files:**
- Modify: `web/index.html` (botões Exportar/Importar na tela de notas, janela de importação)
- Modify: `VERSION`, `web/index.html` (`?v=`), `web/mcp.js` (`version:`)

**Interfaces:**
- Consumes: `CD2Notas.exportar/lerArquivo/mesmoBanco/planoImportacao` (Task 3); `POST /notas/importar` (Task 2); `inst.status.origem` (já carregado em `iniciarInstalacao`).

- [ ] **Step 1: Implement**

- "Exportar" sempre; "Importar" só com `inst.status.origem === 'instalada'`.
- Exportar baixa o arquivo de `CD2Notas.exportar(st.notas, st.notasBanco, new Date())` (mesmo jeito de `downloadCsv`).
- Importar: `<input type="file" accept=".json,application/json">`; `lerArquivo` com erro em tela; se `!mesmoBanco`, aviso "Este arquivo é de outro banco (<motor> <base>). As tabelas podem não bater." com Continuar/Cancelar; depois a janela de `exportar-importar-v1.html`: resumo (notas novas, consultas novas, vão ser sobrescritas), aviso laranja "estes itens já existem aqui e vão ser apagados e substituídos pelo que vem no arquivo. Não dá para desfazer.", tabela com antes riscado e depois, botão "Sobrescrever e importar" (ou "Importar" sem conflito). Confirma → `POST /notas/importar` com `resultado`, recarrega as notas e reaplica no catálogo. 403 → mostra a mensagem do ctl.
- Versão: `0.16.0` → `0.17.0` em `VERSION`, em todos os `?v=` e no `mcp.js`.

- [ ] **Step 2: Run all tests**

Run: `node --test tests/ && (cd ctl && gofmt -l . && go vet ./... && go test ./...)`
Expected: PASS, gofmt sem saída

- [ ] **Step 3: Verificação no app**

No pen drive (origem `pendrive`): Exportar baixa o arquivo, Importar não aparece, e `POST /notas/importar` manual dá 403. Na instalada: importar o arquivo exportado com uma nota alterada aparece como conflito com antes/depois, confirma, e a nota fica a do arquivo; nota que só existia aqui continua. Importar arquivo estragado mostra o erro e nada muda.

- [ ] **Step 4: Commit**

```bash
git add VERSION web/index.html web/mcp.js
git commit -m "Exportar e importar notas e consultas; versão 0.17.0"
```
