# Entendimento guardado e "Atualizar com o banco" — plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** O entendimento do banco é feito uma vez e guardado; o botão "Entendimento do banco" só aparece com ele completo; a nota da tabela vem com o texto da IA; "↻ Atualizar com o banco" entende só o novo e apaga, com aviso, as notas do que sumiu; e um aviso único quando o banco não parece ERP de varejo.

**Architecture:** O cd2-ctl guarda um segundo documento por banco (`entendimento/<chave>.json`) com rotas `GET/POST /entendimento` iguais às de notas. Um módulo JS puro `web/entendimento.js` captura o perfil do catálogo, reaplica, compara catálogo × guardado, limpa notas e reconhece varejo. O `index.html` passa a usar o guardado na abertura, liga o botão ao "completo" e ganha a janela de atualização.

**Tech Stack:** Go 1.23 (cd2-ctl), JavaScript sem build (`node --test`).

**Spec:** [docs/superpowers/specs/2026-10-21-entendimento-guardado-design.md](../specs/2026-10-21-entendimento-guardado-design.md) (continua [2026-10-21-notas-e-consultas-salvas-design.md](../specs/2026-10-21-notas-e-consultas-salvas-design.md))

## Global Constraints

- Esboço aprovado: `.superpowers/brainstorm/8514-1792621823/content/ressincronizar-v1.html` (textos e layout).
- Botão "Entendimento do banco" só aparece com o entendimento completo; a barra de consultas salvas não depende disso.
- Completo = amostras, perfil e fila de descrições terminadas, mesmo com falha do modelo.
- Atualizar descreve só tabelas novas e as sem descrição; nunca redescreve nem mexe em nota existente.
- Comparação de tabelas e campos por nome, sem diferenciar maiúsculas de minúsculas; mudança só de tipo não conta.
- Tabela ou campo com nota que sumiu: nota apagada só depois da janela de aviso confirmada. Consultas salvas afetadas só listadas.
- Caixa da nota: vem com a nota da TI ou, sem ela, com a descrição da IA; só vira nota da TI quando o texto aparado difere da descrição; vazio ou igual ao da IA remove a nota.
- Nenhuma linha de amostra do cliente vai para o disco.
- Entendimento fora da exportação/importação de notas.
- Aviso de varejo: texto exato "Este banco não parece ser de um ERP de varejo. A IA de Bolso foi feita para funcionar melhor com ERPs de varejo: ela vai tentar ajudar mesmo assim, mas pode errar mais."; uma vez por banco, no fim do primeiro entendimento; na dúvida não avisa; sem nome de ERP específico.
- Falha ao gravar o entendimento: botão aparece mesmo assim e nota "Não consegui guardar o entendimento; ele vai ser refeito na próxima abertura."
- Escritas no documento de notas só pela fila `gravarNotas`.
- Versão sobe para **0.18.0** (`VERSION`, todos os `?v=`, `mcp.js`). Suíte JS: `node --test tests/*.test.js tests/*.test.mjs`.

## Review Focus

1. **Banco com nomes em código** (ex.: `ZAN_M45`, `ZAN_M43`) cujas descrições dizem "itens vendidos no cupom": não pode gerar aviso de varejo (teste na Tarefa 2).
2. **Documento guardado de outro formato, estragado ou de versão futura:** a IA refaz a varredura em vez de quebrar (teste na Tarefa 2, `aplicar`).
3. **Tabela renomeada só na caixa** (`VENDAS` → `vendas`): não conta como sumida nem nova (teste na Tarefa 2).
4. **Banco trocado no meio da varredura ou da atualização:** nada é gravado no banco novo (checagem de `catalogGen` e chave; verificação na Tarefa 4/5).
5. **Atualizar com o ctl fora do ar:** erro na janela, nenhuma nota apagada (verificação na Tarefa 5).

---

### Task 1: Documento de entendimento no cd2-ctl

**Files:**
- Modify: `ctl/internal/notas/notas.go` (Store parametrizado por subpasta, vazio e validação)
- Modify: `ctl/internal/api/notas.go`, `ctl/internal/api/api.go`, `ctl/main.go`
- Test: `ctl/internal/notas/notas_test.go`, `ctl/internal/api/notas_test.go`

**Interfaces:**
- Produces:
  - `func NewEntendimento(dir string) *Store` — arquivos em `dir/entendimento/<Chave>.json`, 0600, mesma gravação atômica (CreateTemp + Sync + rename) e `ErrCorrompido` do Store de notas; `const MaxEntendimento = 16 << 20`; vazio `{"versao":1,"tabelas":{}}`; validação: objeto com `versao` 1 e `tabelas` objeto (demais campos livres).
  - `Options.Entendimento *notas.Store`; com nil, as rotas dão 404.
  - `GET /entendimento` → 200 `{"chave":"<32 hex>","dados":{...}}`; 409 sem banco.
  - `POST /entendimento` corpo `{"chave","dados"}` → 200 `{"ok":true}`; 400 sem chave ou dados inválidos; 409 `{"erro":"O banco conectado mudou. Abra a tela de novo."}` com chave diferente; 413 acima de `MaxEntendimento`. Sem rota de importação.

- [ ] **Step 1: Write the failing tests**

```go
func TestEntendimentoLerVazio(t *testing.T)       // NewEntendimento(dir).Ler(b) == `{"versao":1,"tabelas":{}}`
func TestEntendimentoGravarELer(t *testing.T)     // grava {"versao":1,"tabelas":{"vendas":{"papel":"movimento"}},"completo":true}, relê igual, arquivo em dir/entendimento/<Chave>.json
func TestEntendimentoRecusaInvalido(t *testing.T) // `[]`, `{"versao":2,"tabelas":{}}`, `{"versao":1}` → ErrDados
func TestEntendimentoAceitaGrande(t *testing.T)   // 2 MiB válido grava (acima do limite das notas)
// api
func TestEntendimentoRotas(t *testing.T)          // GET devolve chave; POST com chave certa 200; chave de outro banco 409 e nada gravado; sem chave 400; sem banco 409
func TestEntendimentoDesligado(t *testing.T)      // Options sem Entendimento → GET /entendimento 404
```

- [ ] **Step 2: Run to verify they fail** — `cd ctl && go test ./internal/notas/ ./internal/api/` → FAIL
- [ ] **Step 3: Implement** — generalizar o Store (subpasta, vazio, validação e limite como campos), mantendo `New(dir)` das notas com o comportamento de hoje; rotas reaproveitam a leitura/checagem de chave das de notas; `main.go`: `Entendimento: notas.NewEntendimento(*cache)`.
- [ ] **Step 4: Run** — `cd ctl && gofmt -l . && go vet ./... && go test ./...` → PASS, gofmt sem saída
- [ ] **Step 5: Commit** — `git commit -m "cd2-ctl: entendimento do banco guardado por banco"`

---

### Task 2: `web/entendimento.js` — capturar, aplicar, comparar, limpar e varejo

**Files:**
- Create: `web/entendimento.js` (global `CD2Entendimento`, sem DOM, padrão de `web/notas.js`)
- Modify: `web/index.html` (só a tag `<script src="entendimento.js?v=0.17.0">` antes de `ask.js`)
- Test: `tests/entendimento.test.js`

**Interfaces:**
- Consumes: `Cat.tokens`, `Cat.links` de `web/catalog.js`; perfil das tabelas (`t.perfil = { papel, colunas:{data,valor,quantidade}, chave, amostra, descricao? }`).
- Produces:
  - Documento: `{ versao:1, atualizado:ISO, completo:boolean, varejo:{ verificado:boolean, parece:boolean } | undefined, tabelas:{ [nome]:{ papel, colunas, chave, amostra, descricao, campos:[nome...] } } }` (sem `descricaoTokens`, sem linhas de amostra).
  - `capturar(catalog, agora:Date)` → documento com `completo:true` a partir do catálogo atual.
  - `aplicar(catalog, doc)` → `{ ok:boolean, desconhecidas:[nome] }`; com `ok:true` repõe `t.perfil` (e `descricaoTokens = Cat.tokens(descricao, true)`) nas tabelas que estão no documento (casando nome sem caixa). `ok:false` quando o documento não é versão 1, não tem `completo:true` ou está estragado.
  - `comparar(catalog, doc, notas)` → `{ novasTabelas:[nome], novosCampos:[{tabela,campo}], sumiram:{ tabelas:[nome], campos:[{tabela,campo}] }, notasApagadas:[{ tipo:'tabela'|'campo', onde, nota }], semDescricao:[nome], consultasAfetadas:[nome], nadaMudou:boolean }`. `semDescricao`: tabelas do documento com papel em `movimento|itens|indefinida|cadastro` e sem `descricao`. `consultasAfetadas`: consultas de `notas.consultas` cujo `sql` lê (palavra inteira, sem caixa, fora de literais) uma tabela que sumiu. `nadaMudou` só quando não há novidade, sumiço nem `semDescricao`.
  - `limparNotas(notas, diff)` → cópia de `notas` sem as notas das tabelas e campos de `diff.sumiram` (casando sem caixa).
  - `mesclar(doc, catalog, diff, agora)` → documento novo: tira o que sumiu, põe as tabelas novas sem perfil (`{ campos }`), atualiza `campos` de todas pela lista atual e `atualizado`.
  - `pareceVarejo(catalog)` → `{ parece:boolean, grupos:number }` sobre nomes de tabela, nomes de campo e `perfil.descricao`; grupos de sinais (pt e en): produto/item/EAN/GTIN/SKU; venda/cupom/PDV/NFC-e/nota fiscal; estoque; loja/filial; cliente/fornecedor; preço/custo; caixa/operador/finalizadora/pagamento. `parece:false` só com no máximo 1 grupo e nenhuma tabela de papel `movimento` com sinal de venda; o resto é `parece:true` (na dúvida, não avisa).
  - `TEXTO_VAREJO` = o texto exato das Global Constraints.

- [ ] **Step 1: Write the failing tests**

```js
test('capturar e aplicar devolvem o mesmo perfil sem amostras e repõem os tokens', ...);
test('aplicar recusa documento estragado, de outra versão ou incompleto', ...); // null, {versao:2}, {versao:1,tabelas:{}} sem completo → ok:false
test('aplicar lista tabelas do banco que o documento não conhece', ...);
test('comparar ignora caixa: VENDAS no documento e vendas no banco não mudou', ...);
test('comparar acha tabela e campo novos, sumidos, notas que vão ser apagadas e consultas afetadas', ...);
test('comparar: mudança só de tipo não conta', ...);
test('comparar: tabela sem descrição faz nadaMudou false', ...);
test('limparNotas tira só o que sumiu e mantém consultas', ...);
test('mesclar tira o que sumiu, põe as novas sem perfil e atualiza campos', ...);
test('pareceVarejo: ERP com nomes claros (produtos, vendas_cab, lojas) parece', ...);
test('pareceVarejo: nomes em código ZAN_M45/ZAN_M43 com descrições de cupom e itens vendidos parece', ...);
test('pareceVarejo: sistema de clínica (pacientes, consultas, prontuarios, medicos) não parece', ...);
test('pareceVarejo: sistema de RH (funcionarios, folha, ferias, cargos) não parece', ...);
test('pareceVarejo: catálogo vazio ou sem sinal nenhum mas com poucas tabelas não quebra', ...);
```

- [ ] **Step 2: Run** — `node --test tests/entendimento.test.js` → FAIL
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run** — suíte JS inteira → PASS
- [ ] **Step 5: Commit** — `git commit -m "Entendimento: capturar, comparar e reconhecer varejo"`

---

### Task 3: Nota da tabela preenchida com o texto da IA

**Files:**
- Modify: `web/index.html` (tela de notas: `rascunhoDe`, `renderListaNotas`, `renderDetalheNotas`, `salvarTelaNotas`)

**Interfaces:**
- Consumes: `t.perfil.descricao`; `nt.rascunho`; `gravarNotas`.

- [ ] **Step 1: Implement**
  - Caixa "Nota da TI sobre a tabela": mostra a nota da TI do rascunho; sem ela, a descrição da IA; rótulo ganha "(veio da IA, corrija se precisar)" enquanto o texto é o da IA.
  - Ao salvar: para cada tabela do rascunho, se o texto aparado está vazio ou igual à descrição aparada, a nota da tabela é removida (campos continuam); senão é a nota.
  - Lista: "texto da IA" (sem nota da TI e com descrição) | "📝 nota da TI" (com nota da tabela ou de campo) | "sem descrição" (nenhum dos dois).
  - O resto da tela (campos, busca, status, erros, rascunho) igual.
- [ ] **Step 2: Run** — suíte JS → PASS
- [ ] **Step 3: Verificação no app** (ctl compilado do código atual com `sh ctl/build.sh` só se for rodar o app inteiro; ou ctl solto com `--cache` numa pasta temporária, Postgres de teste `docker compose -p cd2eval -f tests/eval/docker-compose.yml up -d --wait erp`, `web/` estático): abrir a tela, ver a caixa com o texto da IA, salvar sem mexer (não vira nota), editar e salvar (vira), apagar tudo e salvar (volta o texto da IA). Parar tudo no fim. Nunca usar a pasta de cache real (`~/Library/Caches/cd2-harness`) nem sobrescrever `bin/ctl`.
- [ ] **Step 4: Commit** — `git commit -m "Tela de notas: nota da tabela vem com o texto da IA"`

---

### Task 4: Entendimento guardado na abertura, botão só com ele completo e aviso de varejo

**Files:**
- Modify: `web/index.html` (`loadCatalog`, `analisarBanco`, `syncBotaoNotas`, faixa, tela de notas)

**Interfaces:**
- Consumes: `GET/POST /entendimento` (Task 1); `CD2Entendimento.aplicar/capturar/pareceVarejo/TEXTO_VAREJO` (Task 2).
- Produces (para a Task 5): `st.entendimento` (documento atual), `st.entendimentoChave`, `st.entendimentoPronto` (boolean), `async gravarEntendimento(doc)` → true|false (com a chave lida no GET; 409 não grava).

- [ ] **Step 1: Implement**
  - `loadCatalog`: depois do catálogo, `GET /entendimento` (timeout 5 s). Se `aplicar` der `ok`, não roda `analisarBanco`; recalcula as sugestões; `st.entendimentoPronto = true`. Senão (ou GET falhou), roda `analisarBanco` como hoje.
  - `analisarBanco`: ao terminar (inclusive com falha do modelo), `capturar`; se o documento guardado não tinha `varejo.verificado`, roda `pareceVarejo`, guarda `{ verificado:true, parece }` e, se `!parece`, `addNote(TEXTO_VAREJO)`; grava com `gravarEntendimento`; falha → nota "Não consegui guardar o entendimento; ele vai ser refeito na próxima abertura."; `st.entendimentoPronto = true` de qualquer jeito.
  - Faixa durante a primeira varredura ganha "O botão "Entendimento do banco" aparece quando terminar."
  - `syncBotaoNotas`: botão só com `podeSalvar() && st.entendimentoPronto`; a barra de consultas salvas continua só com `podeSalvar()`.
  - Tela de notas, com `aplicar(...).desconhecidas.length > 0`: aviso "O banco tem N tabelas que o entendimento ainda não conhece. Clique em Atualizar com o banco."
  - Trocar de banco no meio: nada gravado (`catalogGen`).
- [ ] **Step 2: Run** — suíte JS → PASS
- [ ] **Step 3: Verificação no app** (mesmas regras da Task 3, sem modelo: o modelo falhando conta como completo): primeira abertura faz a varredura, grava e mostra o botão só no fim; recarregar não refaz a varredura e o botão aparece logo; o aviso de varejo não aparece no banco de teste (ERP de varejo) e aparece uma vez num banco que não é (criar um schema simples de clínica no Postgres de teste); ctl parado na hora de gravar mostra a nota de falha.
- [ ] **Step 4: Commit** — `git commit -m "Entendimento guardado, botão só com ele completo e aviso de varejo"`

---

### Task 5: "↻ Atualizar com o banco" e versão 0.18.0

**Files:**
- Modify: `web/index.html` (botão no topo da tela de notas, janela de atualização, entendimento das tabelas novas)
- Modify: `VERSION`, `web/index.html` (`?v=`), `web/mcp.js`

**Interfaces:**
- Consumes: `CD2Entendimento.comparar/limparNotas/mesclar` (Task 2); `st.entendimento`, `gravarEntendimento` (Task 4); `gravarNotas` (fila das notas); `client.call('db_catalog')`, `CD2Catalog.buildCatalog`, `CD2Perfil.amostrar/analisarCatalogo`, `CD2Descricao.executarFila`.

- [ ] **Step 1: Implement**
  - Topo da tela de notas: "↻ Atualizar com o banco" e "atualizado em DD/MM/AAAA" no resumo.
  - Clique: relê `db_catalog`, `buildCatalog`, `comparar(catálogoNovo, st.entendimento, st.notas)`.
    - `nadaMudou`: "O entendimento já está igual ao banco."
    - Senão, janela do esboço: resumo (tabelas novas, campos novos, sumiram); com `notasApagadas`, aviso laranja e tabela com a nota riscada, botão "Apagar notas e atualizar"; sem, botão "Atualizar"; lista de `consultasAfetadas` ("continuam salvas, mas podem dar erro"); frase sobre descrever só as novas.
  - Confirmar: `gravarNotas(atual => limparNotas(atual, diff))` (só se houver nota apagada; falha → erro na janela e para); troca `st.catalog` pelo novo catálogo mantendo o perfil guardado (aplicar o documento mesclado), reaplica as notas; amostra e analisa **só** `novasTabelas ∪ semDescricao` e roda `executarFila` só com elas, com a faixa de progresso, sem esconder botão nem tela; no fim `capturar`/`mesclar` e `gravarEntendimento`.
  - Erros (ctl fora, 409): mensagem na janela, nenhuma nota apagada.
  - Versão `0.17.0` → `0.18.0` em `VERSION`, todos os `?v=` e `mcp.js`.
- [ ] **Step 2: Run** — `node --test tests/*.test.js tests/*.test.mjs && (cd ctl && gofmt -l . && go vet ./... && go test ./...)` → PASS
- [ ] **Step 3: Verificação no app** (regras da Task 3): criar no Postgres de teste uma tabela nova e um campo novo e apagar uma tabela que tem nota; Atualizar mostra o resumo e a nota riscada; confirmar apaga só aquela nota, mantém as outras e as descrições antigas; segunda atualização diz "já está igual"; com ctl parado, erro e nada apagado.
- [ ] **Step 4: Commit** — `git commit -m "Atualizar com o banco; versão 0.18.0"`
