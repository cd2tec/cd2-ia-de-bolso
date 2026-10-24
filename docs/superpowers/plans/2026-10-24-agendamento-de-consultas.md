# Agendamento de consultas salvas — plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Na versão instalada, uma consulta salva roda sozinha numa frequência escolhida, com parâmetros de período calculados a cada execução. Cada execução fica num histórico guardado para sempre e avisa pela notificação do sistema e pela marca no ícone da bandeja. As execuções perdidas viram uma linha com "Rodar agora".

**Architecture:** O relógio fica no **cd2-ctl**. Ele está vivo enquanto a IA está aberta, com ou sem janela, e já é dono do toolbox e das consultas salvas. Para ter **uma regra só**, o cd2-ctl roda os mesmos arquivos JS da tela num interpretador JS embutido em Go ([goja](https://github.com/dop251/goja)), carregados de `<root>/web`. São eles `consultas.js` (montarSql), `toolbox.js` (guardSql e prepareSql), `ask.js` (parseRows, pickVisual e frase) e o novo `agenda.js` (frequência e períodos). O Go só cuida do tempo, da fila, do toolbox (MCP) e dos arquivos. O agendamento fica dentro da consulta salva (`consulta.agenda`, no arquivo de notas que já existe). O histórico fica em JSONL, um arquivo por consulta. O lançador pergunta as novidades ao cd2-ctl a cada 5 s, atualiza a marca e o menu da bandeja e mostra a notificação. O clique abre a conversa com `?agenda=<id>`.

**Tech Stack:** Go 1.25 (cd2-ctl e lançador, `net/http`, `go test`), `github.com/dop251/goja` (novo, só no ctl), JS sem build no padrão `(function(root){...})`, testes com `node --test`, Wails e fyne.io/systray no lançador, AppKit e UserNotifications no Mac, Shell_NotifyIconW no Windows.

**Spec:** [docs/superpowers/specs/2026-10-24-agendamento-de-consultas-design.md](../specs/2026-10-24-agendamento-de-consultas-design.md)

## Global Constraints

- **Agendar e rodar sozinha: só na versão instalada** (`instalacao.Info().Origem == "instalada"`, o mesmo sinal do `PodeImportar`). No pen drive os botões aparecem, e o clique abre o convite com **Agora não** e **Instalar no computador**, que chama `abrirInstalar()`. Nada aparece no modo Demonstração.
- **Intervalo mínimo de 5 minutos.** Folga de **1 minuto** para contar como "na hora". Horário passado fora da folga nunca roda sozinho.
- **Uma execução por vez no ctl inteiro** (fila única). Se a consulta já está na fila ou rodando quando chega o próximo horário, ele vira perda com o motivo `anterior`.
- **Execuções perdidas: uma linha por intervalo.** "Não rodou às HH:MM" quando é um horário só. "Não rodou N vezes entre <dia HH:MM> e <dia HH:MM>" quando são vários. O **Rodar agora** usa o último horário perdido como referência.
- **Motivos da perda e seus textos:** `fechada` = "a IA de Bolso estava fechada", `dormindo` = "o computador estava dormindo", `outro_banco` = "outro banco estava conectado", `anterior` = "a anterior ainda estava rodando". Banco fora do ar não é perda: é execução com erro.
- **Avisos:** toda execução agendada (ok ou erro) gera um aviso. Execução manual não gera aviso nem NOVO. As perdas encontradas ao abrir geram um aviso só: "N execuções não rodaram enquanto a IA estava fechada".
- **Rótulos da bandeja (instalada):** "Fechar (os agendamentos param)" e "Colocar a IA para dormir (os agendamentos param)".
- **Períodos:** todo período é um par início/fim, e um parâmetro sozinho usa o início. A semana vai de **domingo a sábado**. "Esta semana" e "este mês" terminam no último dia do período. "Hora atual" vai do início da hora até a referência. "Últimas X horas" vai da referência menos X até a referência.
- **Mês sem o dia X** roda no último dia do mês.
- **Tipo novo de parâmetro `datahora`** (`AAAA-MM-DD HH:MM:SS`) nos três caminhos: salvar, rodar e importar. O `data` antigo continua valendo.
- **O histórico não é exportado.** A agenda importada chega com `ligado: false`.
- **Apagar a consulta apaga o histórico**, com a confirmação "O histórico de N execuções também será apagado."
- A consulta agendada passa sempre por `montarSql` → `guardSql` → `prepareSql` → `db_read_only_query`, com prazo de **15 s**, igual à tela.
- Os textos de tela seguem os esboços aprovados em `.superpowers/brainstorm/2298-1792864395/content/agendamento-v1.html`, em português do Brasil.
- A versão anda junto: `VERSION`, o `?v=` dos scripts do `index.html` e a `version` do `mcp.js` vão para **0.22.0** na última tarefa.

## Review Focus

1. **Relógio mudado ou horário de verão** (o relógio do computador anda para trás ou pula horas): nada roda em dobro e nada trava. O `ultimoTick` só avança, e o pulo para trás é ignorado (teste na Tarefa 8).
2. **Consulta apagada ou renomeada com a execução na fila:** a execução termina sem gravar em lugar errado, e a consulta apagada não ganha histórico novo (teste na Tarefa 8).
3. **Arquivo de histórico com a última linha cortada** (queda de energia no meio da gravação): a leitura ignora a linha quebrada e o resto continua aparecendo (teste na Tarefa 7).
4. **Banco trocado com o histórico aberto na tela:** a tela fecha o histórico e nada é gravado no banco novo. As rotas conferem a `chave` (teste na Tarefa 9).
5. **Notificação negada no Mac ou app sem assinatura:** a marca e o menu da bandeja continuam funcionando, e o erro só vai para o log (verificação manual na Tarefa 11).

---

### Task 1: Tipo de parâmetro `datahora` em `consultas.js`

**Files:**
- Modify: `web/consultas.js` (`TIPOS`, `acharLiterais`, `paramDeTrecho`, `montarSql`)
- Modify: `web/index.html` (campo da janela Rodar para `datahora`: `<input type="datetime-local" step="1">`; tipo "data e hora" na tabela da janela Salvar)
- Test: `tests/consultas.test.js`

**Interfaces:**
- Produces: `TIPOS = ['data', 'datahora', 'numero', 'texto']`. Um literal `'AAAA-MM-DD HH:MM:SS'` vira `{tipo:'datahora', valor:'2026-10-24 14:00:00', inicio, fim}` cobrindo data e hora. O literal só com data continua `data`. `montarSql` aceita para `datahora` exatamente `AAAA-MM-DD HH:MM:SS`, com data válida, hora de 00 a 23 e minuto e segundo de 00 a 59. Também aceita `AAAA-MM-DDTHH:MM(:SS)?`, que é o que o `datetime-local` manda, e normaliza para o formato com espaço e segundos.
- Consulta salva antiga, com `tipo:'data'` sobre um literal com hora (`fim - inicio === 10`), continua rodando como antes.

- [ ] **Step 1: Write the failing tests**

```js
test('literal com hora vira datahora inteiro', () => {
  const l = C.acharLiterais("SELECT 1 FROM v WHERE dt >= '2026-10-24 14:00:00'");
  assert.deepEqual(l.map(x => [x.tipo, x.valor]), [['datahora', '2026-10-24 14:00:00']]);
});
test('montarSql troca datahora e normaliza o datetime-local', () => {
  const sql = "SELECT 1 FROM v WHERE dt >= '2026-10-24 14:00:00'";
  const p = C.acharLiterais(sql)[0];
  const c = { sql, params:[{ nome:'Desde', tipo:'datahora', inicio:p.inicio, fim:p.fim, valor:p.valor }] };
  assert.equal(C.montarSql(c, ['2026-10-25T08:30']).sql, "SELECT 1 FROM v WHERE dt >= '2026-10-25 08:30:00'");
  assert.equal(C.montarSql(c, ['2026-10-25 24:00:00']).erro, 'Desde: data e hora inválida.');
  assert.equal(C.montarSql(c, ['2026-02-30 10:00:00']).erro, 'Desde: data e hora inválida.');
});
test('parâmetro data antigo sobre literal com hora continua valendo', () => {
  const sql = "SELECT 1 FROM v WHERE dt >= '2026-10-24 14:00:00'", i = sql.indexOf('2026');
  const c = { sql, params:[{ nome:'Dia', tipo:'data', inicio:i, fim:i + 10, valor:'2026-10-24' }] };
  assert.equal(C.montarSql(c, ['2026-10-01']).sql, "SELECT 1 FROM v WHERE dt >= '2026-10-01 14:00:00'");
});
test('paramDeTrecho de um literal com hora inteiro dá datahora', () => { /* seleção do literal inteiro → tipo 'datahora' */ });
```

- [ ] **Step 2: Run** `node --test tests/consultas.test.js`. Expected: FAIL nos quatro testes novos.
- [ ] **Step 3: Implement.** `DATAHORA_RE = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/` e `dataHoraValida(v)`, que reusa `dataValida` na data. A mensagem de erro é `nome + ': data e hora inválida.'`. Atualizar o comentário do topo, que ainda diz "o parâmetro cobre só a data".
- [ ] **Step 4: Run** `node --test`. Expected: PASS em toda a suíte.
- [ ] **Step 5: Commit** `feat(consultas): parâmetro data e hora`

---

### Task 2: `web/agenda.js` — frequência

**Files:**
- Create: `web/agenda.js` (módulo puro, `root.CD2Agenda`, sem DOM)
- Test: `tests/agenda.test.js`

**Interfaces:**
- Formato de `consulta.agenda` (fixo, todas as tarefas usam):

```js
{ ligado:true,
  freq:'minutos'|'horas'|'diario'|'semanal'|'mensal',
  a_cada:1,               // minutos (>=5), horas (>=1), dias (>=1) ou semanas (1|2)
  hora:'07:30',           // diario, semanal, mensal
  janela:{ de:'08:00', ate:'20:00' } | null,   // minutos e horas; ate inclusivo
  dias:[1,2,3,4,5,6],     // 0=domingo; minutos, horas e semanal (vazio = todos)
  dia_mes:15 | 'ultimo',  // mensal
  inicio:'2026-10-24',    // âncora do "a cada X dias" e do "a cada 2 semanas"
  params:[ /* Tarefa 3 */ ] }
```

- Produces:
  - `validar(agenda) -> { ok:true } | { ok:false, erro:string }`. Erros exatos: `'O intervalo mínimo é de 5 minutos.'`, `'Escolha pelo menos um dia.'` (semanal sem dias), `'Horário inválido.'`, `'Dia do mês inválido.'`.
  - `horarios(agenda, de:Date, ate:Date) -> Date[]`: os horários no intervalo aberto em `de` e fechado em `ate` (`de < h <= ate`), em hora local, no máximo `MAX_HORARIOS = 20000`.
  - `proximo(agenda, depois:Date) -> Date | null` (`null` com `ligado:false`).
  - `resumo(agenda, agora:Date) -> string`, por exemplo `'Vai rodar de segunda a sábado, de hora em hora, das 08:00 às 20:00. Próxima: hoje, 15:00.'`.
  - `rotuloCurto(agenda) -> string` para a barra lateral: `'a cada 1 h'`, `'a cada 15 min'`, `'todo dia 07:30'`, `'seg a sáb'`, `'todo dia 15'`, `'último dia do mês'`.
- Regras: `minutos` e `horas` contam a partir de 00:00 de cada dia, ou a partir de `janela.de` quando há janela. Assim, a cada 2 h das 08:00 dá 08, 10, …, 20.

- [ ] **Step 1: Write the failing tests** (use `new Date(2026, 9, 24, 14, 0)`, hora local):

```js
test('a cada 1 h das 08 às 20, seg a sáb', () => {
  const a = { ligado:true, freq:'horas', a_cada:1, janela:{ de:'08:00', ate:'20:00' }, dias:[1,2,3,4,5,6] };
  const h = A.horarios(a, new Date(2026,9,24,6,0), new Date(2026,9,26,9,0)); // sáb 24 → seg 26
  assert.equal(h.length, 13 + 2);           // sáb 08..20, seg 08 e 09; domingo fora
});
test('mensal dia 31 cai no último dia', () => {
  const a = { ligado:true, freq:'mensal', dia_mes:31, hora:'07:00' };
  assert.deepEqual(A.horarios(a, new Date(2026,1,1), new Date(2026,2,1)).map(d => d.getDate()), [28]);
});
test('a cada 2 semanas ancorado no início', () => { /* inicio '2026-10-19' (seg), dias [1] → 19/10, 02/11, 16/11 */ });
test('a cada 3 dias ancorado no início', () => { /* inicio '2026-10-24', hora '07:00' → 24, 27, 30 */ });
test('validar recusa 4 minutos', () => assert.equal(A.validar({ freq:'minutos', a_cada:4 }).erro, 'O intervalo mínimo é de 5 minutos.'));
test('proximo de agenda desligada é null', () => assert.equal(A.proximo({ ligado:false, freq:'horas', a_cada:1 }, new Date()), null));
test('resumo e rótulo', () => { /* textos exatos dos exemplos acima */ });
test('horarios corta em MAX_HORARIOS', () => { /* minutos a cada 5 por 1 ano → length === A.MAX_HORARIOS */ });
```

- [ ] **Step 2: Run** `node --test tests/agenda.test.js`. Expected: FAIL (o módulo não existe).
- [ ] **Step 3: Implement** `web/agenda.js` com as funções acima. Contar em dias de calendário local (`new Date(a, m, d, hh, mm)`), nunca somando milissegundos: o horário de verão não pode deslocar a hora.
- [ ] **Step 4: Run** `node --test`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(agenda): frequência dos agendamentos`

---

### Task 3: `web/agenda.js` — períodos e valores da execução

**Files:**
- Modify: `web/agenda.js`
- Test: `tests/agenda.test.js`

**Interfaces:**
- Consumes: `CD2Consultas.TIPOS` (Tarefa 1).
- `agenda.params[i]` corresponde a `consulta.params[i]`:

```js
{ modo:'fixo', valor:'01' }
{ modo:'periodo', periodo:'hoje'|'ontem'|'hora_atual'|'ultimas_horas'|'ultimos_dias'|'esta_semana'|'semana_passada'|'este_mes'|'mes_passado', n:2, ponta:'inicio'|'fim' }
{ modo:'dia_semana', formato:'0a6'|'1a7'|'sigla' }   // dom=0 | dom=1 | 'DOM','SEG',…,'SAB'
```

- Produces:
  - `PERIODOS`: a lista com `{ id, rotulo, faixa:boolean, so_hora:boolean }` na ordem do esboço. `faixa` marca os períodos de mais de um dia. `so_hora` marca `hora_atual` e `ultimas_horas`.
  - `periodosPara(tipo, temPar:boolean) -> PERIODOS[]`. Para `data`, tira os `so_hora`. Para `data` ou `datahora` sem par, tira os `faixa`. Para `numero` e `texto`, devolve vazio, porque só `fixo` e `dia_semana` se aplicam.
  - `valorDoParam(p, tipo, ref:Date) -> string`. Devolve `AAAA-MM-DD` para `data` e `AAAA-MM-DD HH:MM:SS` para `datahora`.
  - `valoresDaExecucao(consulta, ref:Date) -> { ok:true, valores:string[] } | { ok:false, erro }`. Os valores entram no `montarSql`.
  - `parSugerido(periodo) -> periodo`: o mesmo `periodo`, com `ponta:'fim'`, para o segundo parâmetro de data.

- [ ] **Step 1: Write the failing tests** (`ref = new Date(2026, 9, 24, 14, 37, 12)`, sábado):

```js
const d = (periodo, ponta, tipo='data', n) => A.valorDoParam({ modo:'periodo', periodo, ponta, n }, tipo, ref);
test('dias', () => { assert.equal(d('hoje','inicio'), '2026-10-24'); assert.equal(d('ontem','fim'), '2026-10-23'); });
test('semana de domingo a sábado', () => {
  assert.equal(d('esta_semana','inicio'), '2026-10-18'); assert.equal(d('esta_semana','fim'), '2026-10-24');
  assert.equal(d('semana_passada','inicio'), '2026-10-11'); assert.equal(d('semana_passada','fim'), '2026-10-17');
});
test('mês', () => { assert.equal(d('este_mes','fim'), '2026-10-31'); assert.equal(d('mes_passado','inicio'), '2026-09-01'); });
test('horas', () => {
  assert.equal(d('hora_atual','inicio','datahora'), '2026-10-24 14:00:00');
  assert.equal(d('hora_atual','fim','datahora'), '2026-10-24 14:37:12');
  assert.equal(d('ultimas_horas','inicio','datahora', 2), '2026-10-24 12:37:12');
  assert.equal(d('ultimos_dias','inicio','data', 7), '2026-10-17');
});
test('fim de datahora de período de dia vai até 23:59:59', () => assert.equal(d('hoje','fim','datahora'), '2026-10-24 23:59:59'));
test('dia da semana', () => {
  const v = f => A.valorDoParam({ modo:'dia_semana', formato:f }, 'numero', ref);
  assert.equal(v('0a6'), '6'); assert.equal(v('1a7'), '7'); assert.equal(A.valorDoParam({ modo:'dia_semana', formato:'sigla' }, 'texto', ref), 'SAB');
});
test('valoresDaExecucao mistura fixo e período', () => { /* params [data, data, numero] + agenda.params [hoje inicio, hoje fim, fixo '01'] → ['2026-10-24','2026-10-24','01'] */ });
test('periodosPara', () => { /* data sem par não tem esta_semana nem hora_atual; datahora com par tem tudo */ });
```

- [ ] **Step 2: Run** `node --test tests/agenda.test.js`. Expected: FAIL nos novos.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `node --test`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(agenda): períodos calculados a cada execução`

---

### Task 4: Importar traz a agenda desligada

**Files:**
- Modify: `web/notas.js` (plano de importação, por volta das linhas 130–158)
- Test: `tests/notas.test.js`

**Interfaces:**
- Produces: em `resultado.consultas`, toda consulta que veio do arquivo e tem `agenda` chega com `agenda.ligado === false`. A consulta que só existe aqui mantém a agenda como está. Uma diferença só em `agenda` não conta como conflito: `igualConsulta` ignora `agenda`. Nesse caso a agenda daqui fica.

- [ ] **Step 1: Write the failing tests:** `importar desliga a agenda que vem do arquivo` e `agenda diferente não é conflito e a daqui fica`.
- [ ] **Step 2: Run** `node --test tests/notas.test.js`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `node --test`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(notas): agenda importada chega desligada`

---

### Task 5: Motor JS no cd2-ctl (`ctl/internal/agenda/js.go`)

**Files:**
- Create: `ctl/internal/agenda/js.go`, `ctl/internal/agenda/intl.js` (embutido com `//go:embed`)
- Test: `ctl/internal/agenda/js_test.go` (carrega `../../../web`)
- Modify: `ctl/go.mod` (`go get github.com/dop251/goja@latest`)

**Interfaces:**
- Produces:

```go
type Motor struct{ /* mu sync.Mutex; vm *goja.Runtime */ }
func NovoMotor(webDir string) (*Motor, error) // avalia chart, catalog, notas, toolbox, consultas, ask, agenda nesta ordem, com require('./x.js') → module.exports já avaliado
func (m *Motor) Horarios(consulta json.RawMessage, de, ate time.Time) ([]time.Time, error)
func (m *Motor) Preparar(consulta json.RawMessage, ref time.Time, motor string) (Preparo, error)
type Preparo struct { SQL string; Valores []string; Erro string } // Erro != "" = montarSql ou guardSql recusou (mensagem da tela)
func (m *Motor) Resultado(consulta json.RawMessage, textoTool string, valores []string) (Resultado, error)
type Resultado struct {
	Ok bool `json:"ok"`; Motivo string `json:"motivo,omitempty"` // "sql" | "vazio" | "timeout" (MENSAGENS de ask.js)
	Erro string `json:"erro,omitempty"`
	Visual, X, Y, Titulo, Frase string; Colunas []string; Linhas, Todas []map[string]any; Aviso string
}
```

  (Campos JSON iguais aos de `rodarSalva`, em minúsculas.) `Resultado` reproduz o trecho de `rodarSalva` que vem depois do `runSql`: `parseRows`, a regra do vazio, `pickVisual` e `frase`.
- `intl.js` define `Intl.NumberFormat` só para `'pt-BR'` (`format` com milhar `.` e decimal `,`, respeitando `maximumFractionDigits`), porque o goja não tem `Intl`. Não define nada além do que `chart.js` usa: conferir com `grep -n "Intl\|toLocale" web/*.js`.
- O `Date` do goja usa `time.Local`. A conversão entre `time.Time` e o `Date` do JS é feita em milissegundos Unix.

- [ ] **Step 1: Write the failing tests:**

```go
func TestPrepararUsaPeriodo(t *testing.T)      // consulta com '2026-10-01' marcada e agenda {hoje inicio}; ref 24/10 → SQL com '2026-10-24'
func TestPrepararRecusaSqlNaoSelect(t *testing.T) // sql "DELETE FROM x" → Preparo.Erro começa com "Consulta recusada"
func TestHorariosIgualAoNode(t *testing.T)     // a agenda de horas seg–sáb da Tarefa 2 → 15 horários
func TestResultadoFrase(t *testing.T)         // texto `[{"loja":"A","total":1234.5},{"loja":"B","total":10}]`, visual barra → Frase "Maior: A, 1.234,5. Total: 1.244,5 em 2 categorias."
func TestResultadoVazio(t *testing.T)         // "[]" → Ok false, Motivo "vazio"
func TestMotorSemWeb(t *testing.T)            // NovoMotor("/nao/existe") → erro
```

- [ ] **Step 2: Run** `cd ctl && go test ./internal/agenda/`. Expected: FAIL.
- [ ] **Step 3: Implement.** Uma única `*goja.Runtime` protegida por mutex: o goja não é seguro entre goroutines.
- [ ] **Step 4: Run** `cd ctl && go vet ./... && go test ./...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(ctl): motor JS para rodar as consultas salvas sem a tela`

---

### Task 6: Cliente MCP do toolbox (`ctl/internal/agenda/mcp.go`)

**Files:**
- Create: `ctl/internal/agenda/mcp.go`
- Test: `ctl/internal/agenda/mcp_test.go` (servidor `httptest` falso)

**Interfaces:**
- Produces: `func NovoCliente(url string) *Cliente` e `func (c *Cliente) Consultar(ctx context.Context, sql string) (texto string, err error)`. A sessão MCP segue o mesmo protocolo do `web/mcp.js`: `initialize` com `protocolVersion` igual ao `PROTOCOL_VERSION` de lá, depois `notifications/initialized`, depois `tools/call` com `db_read_only_query`, mandando o cabeçalho `MCP-Protocol-Version` e o `Mcp-Session-Id` quando vier. O `texto` é o `content[0].text` do resultado. `isError: true` vira `texto = "ERRO: " + msg`, que é o que o `parseRows` espera. A sessão é refeita uma vez em 404 ou 400 (toolbox reiniciado).
- Prazo: quem chama passa `context.WithTimeout(…, 15*time.Second)`. Prazo estourado devolve `ErrPrazo`, que vira `Resultado{Ok:false, Motivo:"timeout_front"}` no agendador.

- [ ] **Step 1: Write the failing tests:** `TestConsultarFazInitializeUmaVez`, `TestConsultarErroDoTool`, `TestConsultarRefazSessaoEm404`, `TestConsultarPrazo`.
- [ ] **Step 2: Run** `cd ctl && go test ./internal/agenda/ -run Consultar`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `cd ctl && go test ./...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(ctl): cliente MCP do toolbox`

---

### Task 7: Histórico em disco (`ctl/internal/agenda/historico.go`)

**Files:**
- Create: `ctl/internal/agenda/historico.go`
- Test: `ctl/internal/agenda/historico_test.go`

**Interfaces:**
- Pasta: `<cache>/agenda/<notas.Chave(banco)>/`. Dentro dela ficam `<consultaId>.jsonl` (uma execução por linha, só acrescenta), `<consultaId>.estado.json` (`{"seq":N,"visto":N,"rodadas":{"<seq>":"<RFC3339>"}}`) e `aberta.json` (`{"ate":"<RFC3339>"}`, a última vez que este banco esteve conectado com a IA aberta). Mais `<cache>/agenda/aberta.json`, a última vez que a IA esteve aberta com qualquer banco. Arquivos com modo 0600 e pastas com 0700.
- `consultaId` aceito: `^[A-Za-z0-9_-]{1,64}$`. Fora disso dá `ErrId`, para não haver caminho com `..`.
- Produces:

```go
type Execucao struct {
	Seq      int64      `json:"seq"`
	Tipo     string     `json:"tipo"`            // "agendada" | "manual" | "perda"
	Ref      time.Time  `json:"ref"`             // horário de referência dos períodos
	Em       time.Time  `json:"em"`              // quando rodou (ou quando a perda foi achada)
	Valores  []string   `json:"valores,omitempty"`
	Resultado *Resultado `json:"resultado,omitempty"`
	Motivo   string     `json:"motivo,omitempty"` // perdas: fechada | dormindo | outro_banco | anterior
	De, Ate  *time.Time `json:"de,omitempty"`     // perdas: primeiro e último horário perdido (Ate = Ref)
	N        int        `json:"n,omitempty"`      // perdas: quantos horários
	Rodada   *time.Time `json:"rodada,omitempty"` // perdas: preenchido na leitura a partir do estado
	Novo     bool       `json:"novo"`             // preenchido na leitura: agendada/perda com Seq > visto
}
type Historico struct{ /* dir string; mu sync.Mutex */ }
func NovoHistorico(cache string) *Historico
func (h *Historico) Anexar(chave, id string, e Execucao) (Execucao, error) // atribui Seq = estado.seq+1
func (h *Historico) Pagina(chave, id string, antes int64, n int) (itens []Execucao, mais bool, err error) // mais novo primeiro; antes<=0 = do fim
func (h *Historico) Contar(chave, id string) (int, error)
func (h *Historico) NaoVistas(chave, id string) (int, error)
func (h *Historico) MarcarVisto(chave, id string) error
func (h *Historico) MarcarRodada(chave, id string, seq int64, em time.Time) error
func (h *Historico) Apagar(chave, id string) error
func (h *Historico) Aberta(chave string) (banco, qualquer time.Time, err error) // zero se nunca
func (h *Historico) MarcarAberta(chave string, t time.Time) error            // grava as duas
```

- `Pagina` lê o arquivo de trás para a frente em blocos de 64 KB e não carrega o arquivo inteiro. Linha que não é JSON válido é pulada. `n` vai no máximo até 50.
- Manual não fica `Novo`.

- [ ] **Step 1: Write the failing tests:**

```go
func TestAnexarEPaginar(t *testing.T)        // 120 execuções → Pagina(...,0,50) seq 120..71 e mais=true; Pagina(...,71,50) 70..21; a última página mais=false
func TestLinhaCortadaEPulada(t *testing.T)    // anexa 3, acrescenta `{"seq":4,"ti` sem \n, anexa mais 1 → Pagina devolve 4 válidas
func TestNovoEVisto(t *testing.T)            // 2 agendadas e 1 manual → NaoVistas 2; MarcarVisto → 0 e Novo=false
func TestRodadaNaPerda(t *testing.T)
func TestIdInvalido(t *testing.T)            // "../x" → ErrId
func TestApagar(t *testing.T)                // some o .jsonl e o .estado.json; Contar 0
func TestAberta(t *testing.T)
```

- [ ] **Step 2: Run** `cd ctl && go test ./internal/agenda/ -run 'Anexar|Linha|Novo|Rodada|Id|Apagar|Aberta'`. Expected: FAIL.
- [ ] **Step 3: Implement.** Gravar com `O_APPEND` e terminar cada linha com `\n`. Antes de anexar, se o arquivo não termina em `\n`, acrescentar um `\n`. O estado é gravado com temporário e rename, como o `notas.Store.Gravar`.
- [ ] **Step 4: Run** `cd ctl && go test ./...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(ctl): histórico das execuções agendadas`

---

### Task 8: Agendador (`ctl/internal/agenda/agendador.go`)

**Files:**
- Create: `ctl/internal/agenda/agendador.go`
- Test: `ctl/internal/agenda/agendador_test.go` (relógio, consultar e notas falsos)

**Interfaces:**
- Consumes: `Motor` (Tarefa 5), `Cliente.Consultar` (Tarefa 6), `Historico` (Tarefa 7) e `notas.Store.Ler` com `notas.Banco` e `notas.Chave`.
- Produces:

```go
type Opcoes struct {
	Agora     func() time.Time
	Banco     func() (notas.Banco, bool)            // banco conectado agora
	Consultas func(notas.Banco) (json.RawMessage, error) // notas.Store.Ler
	Consultar func(ctx context.Context, sql string) (string, error)
	Motor     *Motor
	Hist      *Historico
	Tick      time.Duration // 30 s em produção
}
type Agendador struct{ /* ... */ }
func NovoAgendador(o Opcoes) *Agendador
func (a *Agendador) Rodar(ctx context.Context)                 // laço do tick + o executor (fila única)
func (a *Agendador) Passo(agora time.Time)                     // um tick; exportado para o teste
func (a *Agendador) RodarAgora(ctx context.Context, b notas.Banco, id string, perdaSeq int64) (Execucao, error) // manual; espera a vez na fila
func (a *Agendador) Avisos(desde int64) (avisos []Aviso, ultimo int64)
func (a *Agendador) Novidades(b notas.Banco) []Novidade         // por consulta com NaoVistas > 0
type Aviso struct { Seq int64 `json:"seq"`; ConsultaId string `json:"consulta"`; Titulo string `json:"titulo"`; Texto string `json:"texto"` }
type Novidade struct { ConsultaId string `json:"consulta"`; Nome string `json:"nome"`; N int `json:"n"` }
var ErrSemConsulta = errors.New("agenda: consulta não existe")
```

- Regras do `Passo(agora)`:
  1. Sem banco, não faz nada e só grava `<cache>/agenda/aberta.json`.
  2. **Na primeira vez** depois de subir: para cada consulta com agenda ligada, os horários entre `aberta(banco)` e `agora - 1 min` viram **uma** perda. O motivo é `outro_banco` se `qualquer > aberta(banco)` e `fechada` nos outros casos. Se `aberta(banco)` é zero, usa o `agora`, e nada é perdido. Gera **um** `Aviso` com `"N execuções não rodaram enquanto a IA estava fechada"` (sem aviso quando N = 0).
  3. **Nas vezes seguintes:** se `agora - ultimoTick > 3 * Tick`, os horários entre `ultimoTick` e `agora - 1 min` viram uma perda `dormindo` (sem aviso de cada uma). Se `agora < ultimoTick`, o relógio voltou: ignora e não atualiza `ultimoTick`.
  4. Os horários entre `agora - 1 min` e `agora`, ou entre `ultimoTick` e `agora`, se este for mais tarde, vão para a fila. Se a consulta já está na fila ou rodando, viram perda `anterior`.
  5. Grava `MarcarAberta(chave, agora)`.
- Executor: uma goroutine. Na vez de cada item, relê a consulta. Se ela foi apagada ou o banco mudou (`Chave` diferente), descarta sem gravar. Depois `Preparar(ref = horário)` → `Consultar` com prazo de 15 s → `Resultado` → `Anexar(tipo "agendada")` → `Aviso{Titulo: "IA de Bolso · " + nome, Texto: "Rodou às HH:MM · " + (Frase | "N linhas" | Erro | MENSAGENS[Motivo])}`. Erro do `Preparar` também é gravado como execução com `Resultado{Ok:false, Erro}`.
- `RodarAgora` com `perdaSeq > 0` usa `Ref = perda.Ate` e chama `MarcarRodada`. Sem perda, `Ref = agora`. Grava `tipo "manual"` sem aviso. Consulta sem `agenda` também pode, e roda com os valores do último uso (`p.valor`).
- `Avisos` guarda os últimos 100 na memória.

- [ ] **Step 1: Write the failing tests:**

```go
func TestPerdasAoAbrirViramUmaLinha(t *testing.T)   // aberta 20:00 sex, agora 08:00 seg, a cada 5 min → 1 Execucao perda, N=(seg 07:59 - sex 20:00)/5min, motivo fechada, 1 Aviso
func TestPerdaOutroBanco(t *testing.T)              // qualquer > aberta(banco) → motivo outro_banco
func TestComputadorDormiu(t *testing.T)             // ticks 07:00, 07:00:30, depois 09:00 → perda dormindo do 07:30; nada roda atrasado
func TestRelogioVoltou(t *testing.T)                // tick 10:00, depois 09:00 → nada roda nem perde; tick 10:05 roda o das 10:05 uma vez só
func TestNaHoraRoda(t *testing.T)                   // horário 14:00, tick 14:00:40 → roda com ref 14:00 e grava agendada + 1 Aviso com a frase
func TestAnteriorAindaRodando(t *testing.T)         // Consultar bloqueia; próximo horário → perda anterior
func TestFilaUnica(t *testing.T)                    // duas consultas no mesmo horário → Consultar nunca chamado em paralelo
func TestConsultaApagadaNaFila(t *testing.T)        // apagar entre a fila e a vez → nada gravado
func TestRodarAgoraPerda(t *testing.T)              // ref = perda.Ate; MarcarRodada; tipo manual; sem Aviso
func TestAgendaDesligadaNaoRoda(t *testing.T)
```

- [ ] **Step 2: Run** `cd ctl && go test ./internal/agenda/ -run 'Perda|Dormiu|Relogio|NaHora|Anterior|Fila|Apagada|RodarAgora|Desligada'`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `cd ctl && go test -race ./internal/agenda/ && go test ./...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(ctl): agendador das consultas salvas`

---

### Task 9: Rotas `/agenda*` e ligação no `main.go`

**Files:**
- Create: `ctl/internal/api/agenda.go`
- Modify: `ctl/internal/api/api.go` (`Options.Agenda`, `case "/agenda/…"`, e `DELETE` permitido no CORS, o que já acontece)
- Modify: `ctl/main.go` (cria `Motor`, `Cliente` em `http://127.0.0.1:<tbPort>/mcp`, `Historico` e `Agendador`; `go ag.Rodar(ctx)` **só se** `instalacao.Info().Origem == "instalada"`; cancela no encerramento)
- Test: `ctl/internal/api/agenda_test.go`

**Interfaces:**
- `Options.Agenda Agenda` (interface com os métodos do `*agenda.Agendador` e do `*agenda.Historico` usados abaixo). Quando é `nil`, as rotas respondem 404. Fora da instalada: `POST /agenda/rodar` responde 403 `"Agendamento só na versão instalada."`, e as rotas de leitura funcionam.
- Rotas (todas com a chave do banco, como o `gravarDoc`: `?chave=` no GET e `"chave"` no corpo. Chave diferente da do banco conectado dá 409 `"O banco conectado mudou. Abra a tela de novo."`):

| Rota | Entrada | Saída |
|---|---|---|
| `GET /agenda/historico?chave=&consulta=&antes=&n=` | | `{"itens":[Execucao…],"mais":bool,"total":N}` |
| `POST /agenda/rodar` | `{"chave","consulta","perda":seq\|0}` | `Execucao` (espera até 20 s; passou disso, 504) |
| `POST /agenda/visto` | `{"chave","consulta"}` | `{"ok":true}` |
| `DELETE /agenda/historico?chave=&consulta=` | | `{"ok":true}` |
| `GET /agenda/novidades?desde=` | | `{"avisos":[Aviso…],"ultimo":N,"novidades":[Novidade…],"total":N,"agenda":bool}`. Sem banco, a resposta é `novidades:[]`. `agenda` diz se o agendador está ligado. |

- [ ] **Step 1: Write the failing tests:** `TestHistoricoChaveErrada409`, `TestRodarPendrive403`, `TestRodarDevolveExecucao`, `TestNovidadesSemBanco`, `TestDeleteHistorico`, `TestConsultaIdInvalido400`.
- [ ] **Step 2: Run** `cd ctl && go test ./internal/api/ -run Agenda`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `cd ctl && test -z "$(gofmt -l .)" && go vet ./... && GOOS=windows go vet ./... && go test ./...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(ctl): rotas do agendamento`

---

### Task 10: Lançador — novidades, marca e menu da bandeja

**Files:**
- Create: `launcher/internal/novidades/novidades.go` (perguntar ao ctl e montar o estado da bandeja; testável sem a bandeja)
- Test: `launcher/internal/novidades/novidades_test.go`
- Modify: `launcher/bandeja.go` (rótulos e itens novos), `launcher/internal/sessao/sessao.go` (expor `PortaCtl()` e `ChaveCtl()` para o poller, só em `Pronta`), `launcher/internal/icone/icone.go` (marca no ícone do Windows)

**Interfaces:**
- Produces:

```go
type Estado struct { Total int; Linhas []Linha; Avisos []Aviso; Ultimo int64; Agenda bool }
type Linha struct { Consulta, Nome string; N int }
type Aviso struct { Seq int64; Consulta, Titulo, Texto string }
func Perguntar(ctx context.Context, porta int, chave string, desde int64) (Estado, error) // GET /agenda/novidades com Authorization: Bearer, Host 127.0.0.1:<porta> e sem Origin (é GET)
func TextosMenu(e Estado) (topo string, linhas []string) // "3 resultados novos" / "1 resultado novo"; "⏰ Flash de vendas de hoje (2)"; até 5 linhas; topo vazio com Total 0
```

- Poller no `App`: a cada 5 s com a fase `Pronta`. Os avisos novos (`Seq > ultimo`) vão para `notificar` (Tarefa 11). Na primeira pergunta depois de subir, só guarda o `ultimo` e mostra apenas o aviso das perdas, para não repetir notificações antigas.
- Bandeja: **Mac** usa `systray.SetTitle(" 3")` com Total > 0, ou `""`. **Windows** usa `icone` com um círculo vermelho no canto superior direito quando Total > 0, sem número. No menu entram um item de topo (desligado, só texto) e 5 itens de linha pré-criados, escondidos (`Hide()`) quando não usados, acima de "Mostrar a IA de Bolso". Clicar numa linha chama `a.abrirAgenda(consulta)`.
- Na instalada, os rótulos `itemDormir` e `itemFechar` ganham `" (os agendamentos param)"`. No pen drive não mudam.

- [ ] **Step 1: Write the failing tests:** `TestPerguntarMandaChave`, `TestTextosMenuPlural`, `TestTextosMenuCortaEm5`, `TestPerguntarCtlForaDoAr` (erro, sem pânico).
- [ ] **Step 2: Run** `cd launcher && go test ./internal/novidades/`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `cd launcher && test -z "$(gofmt -l .)" && go vet ./internal/... && GOOS=windows go vet . && go test ./internal/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(lancador): novidades dos agendamentos na bandeja`

---

### Task 11: Lançador — notificação do sistema e abrir o histórico

**Files:**
- Modify: `launcher/internal/navegador/navegador.go` (`ComAgenda`), `launcher/janela.go` (`abrirAgenda`), `launcher/bandeja_windows.go`, `launcher/bandeja_darwin.go`, `launcher/bandeja_darwin.m`, `launcher/bandeja_outros.go` (stub)
- Test: `launcher/internal/navegador/navegador_test.go`

**Interfaces:**
- `func ComAgenda(url, consulta string) string`: põe `agenda=<consulta>` antes do `#`, igual ao `ComInstalar`. O `consulta` é escapado com `url.QueryEscape`.
- `func (a *App) abrirAgenda(consulta string)`: chama `navegador.Abrir(navegador.ComAgenda(urlDaSessao, consulta))`. Só em `Pronta`.
- `func notificar(titulo, texto, consulta string)`, uma por sistema:
  - **Windows:** `NIF_INFO` no ícone 100, como em `avisarPrimeiroFechar`. Para saber do clique, subclassifica a janela `SystrayClass` (`SetWindowLongPtrW(GWLP_WNDPROC)` com `windows.NewCallback`), trata `NIN_BALLOONUSERCLICK` (0x405) na mensagem de callback do systray e passa o resto para a WndProc original. O `consulta` do último balão fica num campo. Clique: `abrirAgenda`.
  - **Mac:** `UNUserNotificationCenter`. Pede autorização uma vez no início (só alerta). O `userInfo` leva `consulta`, e um delegate com `didReceiveNotificationResponse` chama o Go por `//export notificacaoClicada(*C.char)`. Sem permissão ou com erro, `NSLog` e segue.
  - **Outros:** nada.

- [ ] **Step 1: Write the failing test:** `TestComAgenda` (`ComAgenda("http://127.0.0.1:8080/?v=1#k=abc", "c17") == "http://127.0.0.1:8080/?v=1&agenda=c17#k=abc"`, e `"a b"` vira `a+b`).
- [ ] **Step 2: Run** `cd launcher && go test ./internal/navegador/`. Expected: FAIL.
- [ ] **Step 3: Implement** `ComAgenda` e depois as notificações.
- [ ] **Step 4: Run** `cd launcher && go test ./internal/... && GOOS=windows go vet .`. Expected: PASS.
- [ ] **Step 5: Verify manually** no Mac (`wails build` do lançador): agendar uma consulta a cada 5 minutos e fechar a janela. Em até 5 min aparecem a notificação e o " 1" na barra de menus. O clique abre o histórico. Negar a permissão de notificação: a marca continua. Repetir no Windows se houver máquina, ou anotar no PR que não foi verificado lá.
- [ ] **Step 6: Commit** `feat(lancador): notificação dos agendamentos`

---

### Task 12: Tela — barra lateral, menu "⋯", janela Agendar e convite do pen drive

**Files:**
- Modify: `web/index.html` (CSS `.salvas__*`, a lista da barra lateral por volta da linha 2274, o menu "⋯" e um `<dialog id="agendar">` e um `<dialog id="convAgenda">` novos; carregar `agenda.js?v=…` antes de `ask.js`)
- Test: `tests/agenda.test.js` (só a parte pura que a tela usa, se surgir alguma)

**Interfaces:**
- Consumes: `CD2Agenda.validar`, `horarios`, `proximo`, `resumo`, `rotuloCurto`, `periodosPara`, `parSugerido` e `PERIODOS`. A gravação usa o `POST /notas` que já existe, com `consulta.agenda`. O pen drive é identificado pelo `inst` e pela tag `pPendrive` que já existem.
- Comportamento (textos exatos do esboço `agendamento-v1.html`):
  - Item agendado: a linha `⏰ <rotuloCurto> · próxima <HH:MM ou dd/mm HH:MM>` e a bolinha com `NaoVistas` (vem de `GET /agenda/novidades`, que a tela pergunta a cada 30 s e ao voltar o foco).
  - O filtro `Todas` / `⏰ Só agendadas (N)` só aparece com N > 0.
  - Menu "⋯": `▶ Rodar agora`, `⏰ Agendar…`, `🗂 Ver histórico` (só com agenda ou histórico), `✎ Renomear`, `🗑 Apagar`.
  - Janela Agendar: os botões de frequência, os campos de cada frequência, os dias, a tabela de parâmetros (período ou valor fixo; `dia_semana` para número e texto) e o resumo atualizado a cada mudança. Também a caixa "Agendamento ligado" e os botões **Cancelar**, **Tirar agendamento** (só se já havia agenda) e **Salvar agendamento**. O erro do `validar` aparece embaixo e bloqueia o salvar. Ao escolher um período de faixa no primeiro parâmetro de data, o segundo recebe o `parSugerido`.
  - Pen drive: `⏰ Agendar…` abre o `convAgenda`. "Instalar no computador" fecha a janela e chama `abrirInstalar()`.

- [ ] **Step 1: Implement.**
- [ ] **Step 2: Run** `node --test`. Expected: PASS.
- [ ] **Step 3: Verify manually** com `INICIAR-CD2-Mac.command` (instalada simulada ou de verdade) e um banco de teste: agendar "a cada 5 minutos", conferir o resumo e a próxima execução, reabrir a janela e ver os valores guardados. No pen drive, conferir o convite.
- [ ] **Step 4: Commit** `feat(tela): agendar consulta salva`

---

### Task 13: Tela — histórico, Rodar agora pelo ctl, `?agenda=` e apagar

**Files:**
- Modify: `web/index.html`

**Interfaces:**
- Consumes: as rotas da Tarefa 9 e `renderAnswer` para desenhar o `Resultado` de cada execução como card (mesma forma do `rodarSalva`).
- Comportamento:
  - "Ver histórico" ou o clique numa consulta agendada (na instalada) abre o histórico na área do `#chat`, com o cabeçalho "← Voltar para a conversa", o nome, a pílula com o `rotuloCurto` e os botões **✎ Editar agendamento** e **▶ Rodar agora**. Voltar restaura a conversa como estava (guardar e devolver os nós do `#chat`).
  - Lista: `GET /agenda/historico` com `n=20` e mais 20 ao chegar perto do fim (IntersectionObserver), separada por dia ("Hoje, 24/10", "Ontem, 23/10", "sex, 18/10"). Só a primeira execução vem aberta, e as outras mostram "▾ Mostrar resultado". O card tem a marca `NOVO` quando `novo`. Tipo e hora: "Rodou às 14:00 (agendada)", "Rodou às 15:04 (referente a 12:00)".
  - A perda aparece como card laranja, com "⚠ Não rodou às 12:00" ou "⚠ Não rodou 812 vezes entre sex 20:00 e seg 07:55", o texto do motivo e o botão **▶ Rodar agora**, ou "rodada às 15:04" quando `rodada`.
  - Execução com erro: card com a mensagem e **▶ Rodar agora**.
  - Ao abrir: `POST /agenda/visto` e a bolinha some.
  - **Rodar agora** de uma consulta agendada (na barra, no cabeçalho ou numa perda) usa `POST /agenda/rodar`. O card aparece onde foi clicado: na conversa quando veio da barra lateral, no topo do histórico quando veio do histórico. A consulta não agendada continua no `rodarSalva` de hoje.
  - `?agenda=<id>` na URL: depois de conectar ao banco, abre o histórico daquela consulta e tira o parâmetro com `history.replaceState` (o `#k=` fica).
  - Apagar consulta com histórico: `Contar` vem no `total` do histórico, e a confirmação diz "O histórico de N execuções também será apagado.". Depois do `POST /notas`, chama `DELETE /agenda/historico`.
  - Banco trocado (409 em qualquer rota `/agenda`): fecha o histórico e volta para a conversa.

- [ ] **Step 1: Implement.**
- [ ] **Step 2: Run** `node --test`. Expected: PASS.
- [ ] **Step 3: Verify manually:** com a IA fechada por 20 min e uma consulta a cada 5 min, reabrir e ver **uma** perda "Não rodou 4 vezes entre …" e uma notificação só. Rodar agora nela e conferir o "(referente a …)" e o "rodada às". Apagar a consulta e conferir a confirmação com o número. Trocar de banco com o histórico aberto.
- [ ] **Step 4: Commit** `feat(tela): histórico dos agendamentos`

---

### Task 14: Versão 0.22.0 e verificação final

**Files:**
- Modify: `VERSION`, `web/index.html` (todos os `?v=`), `web/mcp.js` (`CLIENT_INFO.version`), `README.md` (uma linha sobre agendamento na lista do que a versão instalada faz, se houver essa lista)

- [ ] **Step 1:** Trocar `0.21.0` por `0.22.0` nos três lugares (`grep -rn "0\.21\.0" VERSION web/`, que deve ficar vazio no fim).
- [ ] **Step 2: Run** a CI local: `cd ctl && test -z "$(gofmt -l .)" && go vet ./... && go test ./...`, depois `cd launcher && test -z "$(gofmt -l .)" && go vet ./internal/... && go test ./internal/...` e `node --test`. Expected: tudo PASS.
- [ ] **Step 3: Commit** `chore: versão 0.22.0 (agendamento de consultas)`
