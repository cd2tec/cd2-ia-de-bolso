# Planilhas do Excel como banco — plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Um motor novo, `excel`, que copia as planilhas `.xlsx`/`.xlsm` de uma pasta para o SQLite que o toolbox já consulta no DBF, com cabeçalho procurado, abas ignoradas listadas em Configurações e o botão Atualizar relendo a pasta (Excel e DBF).

**Architecture:** O `cd2-ctl` ganha um pacote `copia` (identificador transliterado + gravação SQLite, extraído do `dbf`) e um pacote `excel` (leitura das abas com excelize, núcleo puro sobre uma grade de células). O supervisor passa a importar em duas etapas (`.novo` e troca) e ganha `Reload`; o controlador ganha `Reler` e o relatório `planilhas` no status; a API ganha `POST /reler`. No front, o motor entra em `toolbox.js`, `ask.js` e `index.html`, e a janela de Atualizar ganha a etapa "relendo", que reabre a sessão MCP sem descartar catálogo, notas e entendimento.

**Tech Stack:** Go 1.23 (`ctl/`), `github.com/xuri/excelize/v2` v2.11.0, `modernc.org/sqlite`, `golang.org/x/text`; front em JS puro testado com `node --test`.

**Spec:** [docs/superpowers/specs/2026-10-23-planilhas-excel-design.md](../specs/2026-10-23-planilhas-excel-design.md)

## Global Constraints

- Valor interno do motor: `excel`. Rótulo na tela: `Planilhas Excel`. Rótulo do campo da base: `Pasta das planilhas`. Placeholder da base: `ex.: C:\Controle da loja`; host e usuário `ex.: nao usado`; porta 0; schema vazio.
- Só `.xlsx` e `.xlsm` (sem distinguir maiúsculas). Ficam de fora: subpastas, nomes começando em `~$` ou `.`, outras extensões.
- Nunca escrever na planilha da pessoa; nunca seguir link externo; excelize só lê valores gravados.
- Mensagens para a pessoa em português, no tom das do DBF (frases curtas, sem jargão, sem nome de biblioteca).
- Cópia SQLite em `<CacheDir>/excel.sqlite`; variável `CD2_DB_SQLITE` como no DBF; receita `core/toolbox/excel.yaml`; o supervisor escolhe `engine + ".yaml"`.
- Datas como texto `AAAA-MM-DD`, ou `AAAA-MM-DD HH:MM:SS` quando houver hora (igual ao DBF).
- Nome de tabela e coluna: até 60 caracteres, só `[A-Za-z0-9_]`, prefixo `T_` quando vazio ou começa com dígito, colisões com `_2`, `_3`.
- Versão final `0.21.0` em `VERSION`, nos `?v=` do `web/index.html` e em `web/mcp.js`.
- CI roda `gofmt`, `go vet`, `go test ./...` e `go test -tags sandbox ./...` em `ctl/`, e `node --test` na raiz. Tudo verde a cada commit.
- Commits em português, no estilo do repositório, terminando com `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`.

## Review Focus

Entradas que o design implica e que nenhum teste cobriria sem as linhas abaixo. Cada uma tem o teste pedido na tarefa dona:

1. **Aba com título mesclado ocupando várias colunas na linha 1 e cabeçalho na linha 2** (célula mesclada preenche só a primeira coluna): o cabeçalho é a linha 2, não a 1. Teste na Tarefa 2.
2. **Coluna de datas com uma célula de texto no meio** ("a confirmar"): coluna inteira vira `TEXT`, e as datas aparecem como o Excel mostra, não como número serial. Teste na Tarefa 3.
3. **Dois arquivos com o mesmo nome em caixa diferente** (`Vendas.xlsx` e `vendas.xlsx`, possível no Mac e no Linux): tabelas `Vendas` e `vendas_2`, sem erro. Teste na Tarefa 3.
4. **Releitura em que a importação falha** (pasta sumiu entre uma e outra): toolbox anterior continua rodando, status `rodando` com `erro` preenchido, cópia antiga intacta. Teste na Tarefa 5.
5. **Atualizar com a sessão MCP reaberta e a janela fechada no meio** (a pessoa clica fora enquanto relê): nada é gravado e a próxima abertura da janela recomeça do zero. Teste manual na Tarefa 7, com o guarda `leitura !== at.leitura` já existente.

---

### Tarefa 1: pacote `copia` (identificador transliterado e gravação SQLite), DBF migra para ele

**Files:**
- Create: `ctl/internal/copia/copia.go`, `ctl/internal/copia/copia_test.go`
- Modify: `ctl/internal/dbf/import.go` (tira `identificador`, `unico`, `gravar`, `quote`, `coluna`, `tabela`; passa a usar `copia`)
- Test: `ctl/internal/dbf/import_test.go` (continua passando sem mudança)

**Interfaces:**
- Produces:
  - `type Coluna struct { Nome, Tipo string }`
  - `type Tabela struct { Nome string; Colunas []Coluna; Linhas [][]any }`
  - `func Identificador(s string) string`
  - `func Unico(nome string, usados map[string]int) string`
  - `func Gravar(destino string, tabelas []Tabela) error` — apaga `destino`, `-wal` e `-shm`, cria o diretório, grava tudo numa transação; erro já em português ("Não consegui preparar a cópia.", "Não consegui gravar a cópia.", "Não consegui copiar <nome>.").

- [ ] **Passo 1: escrever os testes de `Identificador` e `Unico`**

```go
func TestIdentificadorTranslitera(t *testing.T) {
	casos := map[string]string{
		"Descrição": "Descricao", "Preço (R$)": "Preco_R", "Nº": "N", "Valor Total": "Valor_Total",
		"data-venda.1": "data_venda_1", "01/09": "T_01_09", "2024": "T_2024", "": "T_", "a  b": "a_b",
		strings.Repeat("x", 70): strings.Repeat("x", 60),
	}
	for in, want := range casos {
		if got := Identificador(in); got != want { t.Errorf("%q: %q != %q", in, got, want) }
	}
}
func TestUnicoIgnoraCaixa(t *testing.T) {
	u := map[string]int{}
	if Unico("Vendas", u) != "Vendas" || Unico("vendas", u) != "vendas_2" || Unico("VENDAS", u) != "VENDAS_3" { t.Fatal() }
}
func TestGravarCriaTabelasETipos(t *testing.T) // grava uma Tabela com colunas TEXT/NUMERIC/DATE e 2 linhas num destino em t.TempDir()/sub/x.sqlite; confere pragma_table_info e COUNT(*)
```

- [ ] **Passo 2: rodar e ver falhar**

Run: `cd ctl && go test ./internal/copia/`
Expected: FAIL (pacote não existe)

- [ ] **Passo 3: implementar `copia.go`**

`Identificador`: `norm.NFD` + remover runas `unicode.Mn` (de `golang.org/x/text/unicode/norm`); depois a regra do DBF de hoje com duas mudanças: `/` vira `_` junto com espaço, hífen e ponto; underscores repetidos colapsam num só. `Unico` e `Gravar` são o `unico`/`gravar` do `dbf` de hoje movidos, com `Gravar` abrindo e fechando o SQLite por conta própria (o `Import` do DBF deixa de abrir o banco).

- [ ] **Passo 4: migrar `dbf/import.go` para `copia`**

`ler` devolve `copia.Tabela`; `Import` monta `[]copia.Tabela` e chama `copia.Gravar(destino, tabelas)`. Nome da tabela: `copia.Unico(copia.Identificador(...), usados)`. Mensagens do DBF ficam como estão.

- [ ] **Passo 5: rodar tudo**

Run: `cd ctl && gofmt -l . && go vet ./... && go test ./internal/copia/ ./internal/dbf/`
Expected: `gofmt` sem saída; PASS nos dois pacotes.

- [ ] **Passo 6: commit**

```bash
git add ctl/internal/copia ctl/internal/dbf
git commit -m "cd2-ctl: pacote copia (identificador com acento transliterado e gravação SQLite); DBF usa ele"
```

---

### Tarefa 2: núcleo do leitor de aba (cabeçalho, largura, linhas, tipos) sobre uma grade de células

**Files:**
- Create: `ctl/internal/excel/aba.go`, `ctl/internal/excel/aba_test.go`

**Interfaces:**
- Consumes: `copia.Tabela`, `copia.Coluna`, `copia.Identificador`, `copia.Unico`.
- Produces:
  - `type Tipo int` com `Vazia, Texto, Numero, Data, Bool, Erro`
  - `type Celula struct { Tipo Tipo; Texto string; Num float64; Data time.Time }` — `Texto` é sempre o valor formatado como o Excel mostra; `Num` vale em `Numero`; `Data` vale em `Data`.
  - `const MotivoSemCabecalho = "sem cabeçalho que eu reconheça"`, `MotivoVazia = "aba vazia"`
  - `func analisarAba(grade [][]Celula) (colunas []copia.Coluna, linhas [][]any, motivo string)` — `motivo` vazio quando deu certo. Nomes de coluna já passados por `copia.Identificador` e `copia.Unico`.

- [ ] **Passo 1: escrever os testes com grades montadas à mão**

Ajudantes no teste: `txt(s)`, `num(f)`, `data(ano, mes, dia)`, `vazia()`.

```go
func TestCabecalhoNaPrimeiraLinha(t *testing.T)
// grade: [Produto, Qtde, Data] / [Arroz, 2, 2026-09-01] / [Feijão, 1.5, 2026-09-02]
// colunas: Produto TEXT, Qtde NUMERIC, Data DATE; linhas[0] == {"Arroz", 2.0, "2026-09-01"}

func TestPlanilhaViva(t *testing.T)
// linha1: [Controle de Vendas 2026] (uma célula só) / linha2: vazia / linha3: [Data, vazia, Valor]
// linha4: [2026-09-01, Centro, 10] / linha5: vazia / linha6: [2026-09-02, Bairro, 20] / linha7: [Total, vazia, 30]
// colunas: Data DATE, Coluna_B TEXT, Valor INTEGER; 2 linhas; nenhuma linha com "Total"

func TestTituloMescladoLargoECabecalhoNaLinha2(t *testing.T)
// linha1: [Relatório, vazia, vazia] (mesclado: só a primeira célula tem valor) / linha2: [Nome, Qtde, Preço (R$)] / linha3: [a, 1, 2.5]
// cabeçalho é a linha 2: colunas Nome, Qtde, Preco_R

func TestCabecalhoComAnoNasColunas(t *testing.T)
// [Produto, 2024, 2025] (2024 e 2025 como Numero com Texto "2024"/"2025") / [Arroz, 10, 12]
// colunas: Produto, T_2024, T_2025

func TestSoUmaCelulaDeTextoNaoECabecalho(t *testing.T)
// [Vendedor, 100, 200] / [Ana, 1, 2]  → motivo MotivoSemCabecalho (só 1 célula de texto)

func TestColunaMistaViraTexto(t *testing.T)
// [Obs] / [10 (Numero, Texto "10")] / [dez]  → Obs TEXT; linhas {"10"}, {"dez"}

func TestInteiroNumericoBoolErro(t *testing.T)
// colunas todas inteiras → INTEGER; com 1.5 → NUMERIC; booleanos → INTEGER 1/0; célula Erro → nil

func TestSubtotalComAcentoSai(t *testing.T)
// linha cuja primeira célula de texto é "SubTotal geral" ou "TOTAL" sai; "Totalmente ok" (começa com total) também sai — documentar no comentário do teste que é a regra escolhida

func TestAbaVaziaESemCandidata(t *testing.T)
// grade vazia → MotivoVazia; 30 linhas de número puro → MotivoSemCabecalho

func TestCabecalhoDepoisDaLinha30NaoConta(t *testing.T)
// cabeçalho válido só na linha 31 → MotivoSemCabecalho
```

- [ ] **Passo 2: rodar e ver falhar**

Run: `cd ctl && go test ./internal/excel/ -run 'Cabecalho|Planilha|Titulo|SoUma|Coluna|Inteiro|Subtotal|Aba'`
Expected: FAIL (não compila)

- [ ] **Passo 3: implementar `aba.go`**

Regras, na ordem do design:

- Candidata (entre as linhas 1–30): ≥ 2 células não vazias, ≥ 2 delas `Texto`, e a próxima linha não vazia a no máximo 2 linhas de distância tem ≥ 1 célula não vazia dentro da largura. Primeira candidata vence.
- Largura: da primeira à última célula não vazia do cabeçalho. Nome: `Texto` da célula (número e data também usam `Texto`); vazia → `Coluna_` + letra da coluna (`excelize.ColumnNumberToName`, ou cópia local da conversão para não depender da excelize neste arquivo: escolha do construtor). Tudo por `copia.Identificador` e `copia.Unico`.
- Linhas: abaixo do cabeçalho até o fim; pula linha vazia dentro da largura; pula linha cuja primeira célula não vazia é `Texto` que, sem acento (reusar a decomposição de `copia`) e em minúsculas, começa com `total` ou `subtotal`; ignora células fora da largura.
- Tipo por coluna sobre as células não vazias (`Erro` conta como vazia): todas `Numero` → `INTEGER` se todas inteiras, senão `NUMERIC`; todas `Data` → `DATE`; todas `Bool` → `INTEGER`; nenhuma célula → `TEXT`; mistura → `TEXT`.
- Valor: `Numero` → `Num` (int64 quando a coluna é `INTEGER`); `Data` → texto `2006-01-02`, ou `2006-01-02 15:04:05` se tiver hora/minuto/segundo; `Bool` → 1/0; `Texto` e qualquer célula numa coluna `TEXT` → `Texto`; vazia e `Erro` → `nil`.

- [ ] **Passo 4: rodar e ver passar**

Run: `cd ctl && go test ./internal/excel/`
Expected: PASS

- [ ] **Passo 5: commit**

```bash
git add ctl/internal/excel
git commit -m "cd2-ctl: núcleo do leitor de aba de planilha (cabeçalho procurado, total fora, tipos por coluna)"
```

---

### Tarefa 3: leitura dos arquivos com excelize e `Import` com relatório

**Files:**
- Modify: `ctl/go.mod`, `ctl/go.sum` (`go get github.com/xuri/excelize/v2@v2.11.0`)
- Create: `ctl/internal/excel/import.go`, `ctl/internal/excel/import_test.go`

**Interfaces:**
- Consumes: `analisarAba`, `Celula`, `copia.Gravar`, `copia.Identificador`, `copia.Unico`.
- Produces:
  - `type Ignorada struct { Arquivo string \`json:"arquivo"\`; Aba string \`json:"aba"\`; Motivo string \`json:"motivo"\` }`
  - `type Relatorio struct { Arquivos int \`json:"arquivos"\`; Tabelas int \`json:"tabelas"\`; Ignoradas []Ignorada \`json:"ignoradas"\`; LidoEm time.Time \`json:"lido_em"\` }` — `Ignoradas` nunca nula no JSON (lista vazia).
  - `func Import(pasta, destino string) (*Relatorio, error)` — o relatório vem também quando `err != nil` (nulo só quando a pasta não existe ou não é pasta).
  - Motivos: `MotivoOculta = "aba oculta"`, `MotivoNaoAbriu = "o arquivo não abriu"`, `MotivoSenha = "arquivo protegido por senha"` (mais os dois da Tarefa 2). Linha de arquivo inteiro ignorado tem `Aba` vazia.
  - Mensagens de erro (texto exato): `Não achei a pasta das planilhas.`, `A base das planilhas é uma pasta com os arquivos .xlsx, não um arquivo.`, `Não achei planilha .xlsx ou .xlsm nessa pasta.` (+ ` Encontrei arquivos .xls/.csv, que ainda não leio. Abra cada um no Excel e salve como .xlsx.` quando houver `.xls` ou `.csv`), `Nenhuma aba das planilhas dessa pasta tem um cabeçalho que eu reconheça.`

- [ ] **Passo 1: escrever os testes, gerando os `.xlsx` com a própria excelize em `t.TempDir()`**

Ajudante `novaPlanilha(t, caminho, func(f *excelize.File))` que cria, preenche e salva.

```go
func TestImportBemComportada(t *testing.T)
// clientes.xlsx, aba única "Plan1": A1:C1 = Nome, Limite, Desde; 2 linhas; C com estilo NumFmt 14 e valor time.Time
// tabela "clientes" (sem sufixo de aba); tipos TEXT/NUMERIC/DATE; SELECT Desde == "2026-01-15"; Relatorio{Arquivos:1, Tabelas:1, Ignoradas vazia}

func TestImportVivaComDuasAbasEUmaOculta(t *testing.T)
// "vendas 2026.xlsx": aba "Setembro" (título mesclado A1:C1, cabeçalho na linha 3 com B3 vazia, linha em branco, linha Total), aba "Outubro" (simples), aba "Rascunho" oculta (SetSheetVisible false)
// tabelas vendas_2026__Setembro e vendas_2026__Outubro; Ignoradas == [{"vendas 2026.xlsx","Rascunho",MotivoOculta}]; Coluna_B presente; COUNT(*) de Setembro sem a linha Total

func TestImportAbaSoComTextoSolto(t *testing.T)
// aba com A1 = "anotações" e A5 = "ligar para o João": Ignorada com MotivoSemCabecalho; o arquivo tem outra aba boa, então Tabelas == 1

func TestImportColunaDeDataComTextoNoMeio(t *testing.T)
// coluna com duas datas (NumFmt 14) e "a confirmar": tipo TEXT; os valores das datas são o texto formatado (ex.: "01/15/26" ou o que o formato 14 der), não "46037"
// confere apenas que não é só dígitos: regexp `^\d+(\.\d+)?$` não casa

func TestImportFormulaSemValorGravadoVemVazia(t *testing.T)
// SetCellFormula("C2", "A2*B2") sem valor em cache (a excelize não calcula ao salvar): a célula vem nil e a coluna C, com só essa célula, é TEXT
// (o caminho "fórmula com valor gravado" entra pelo GetRows normal e não precisa de teste próprio)

func TestImportIgnoraTravaXlsEDicaDeSalvarComoXlsx(t *testing.T)
// pasta com "~$vendas.xlsx" (bytes quaisquer), "antigo.xls" (bytes quaisquer), "dados.csv" e nenhum .xlsx válido
// err contém "Não achei planilha .xlsx ou .xlsm nessa pasta." e "salve como .xlsx"; a trava não aparece em Ignoradas

func TestImportArquivoCorrompidoNaoDerruba(t *testing.T)
// "ruim.xlsx" com bytes aleatórios + "bom.xlsx" válido: err == nil; Ignoradas == [{"ruim.xlsx","",MotivoNaoAbriu}]; Tabelas == 1

func TestImportProtegidoPorSenha(t *testing.T)
// f.SaveAs(caminho, excelize.Options{Password:"x"}) → Ignorada com MotivoSenha

func TestImportTudoIgnoradoDevolveRelatorio(t *testing.T)
// só um arquivo com uma aba de texto solto: err == "Nenhuma aba das planilhas dessa pasta tem um cabeçalho que eu reconheça."; rel != nil; len(rel.Ignoradas) == 1

func TestImportNomesIguaisEmCaixaDiferente(t *testing.T)
// "Vendas.xlsx" e "vendas.xlsx" (pular com t.Skip se o sistema de arquivos não distinguir): tabelas "Vendas" e "vendas_2"

func TestImportPastaInexistenteENaoPasta(t *testing.T)
// mensagens exatas; rel == nil
```

- [ ] **Passo 2: rodar e ver falhar**

Run: `cd ctl && go get github.com/xuri/excelize/v2@v2.11.0 && go test ./internal/excel/ -run Import`
Expected: FAIL (`Import` não definido)

- [ ] **Passo 3: implementar `import.go`**

- `arquivos(pasta)`: como no DBF, filtrando extensão, `~$`, `.`; separadamente conta `.xls`/`.csv` para a dica.
- Por arquivo: `excelize.OpenFile(caminho)`; erro `errors.Is(err, excelize.ErrWorkbookPassword)` → `MotivoSenha`; outro erro → `MotivoNaoAbriu`. `f.GetWorkbookProps()` para `Date1904`.
- Por aba de `f.GetSheetList()`: `f.GetSheetVisible(nome)` falso → `MotivoOculta`. Senão montar a grade: `f.GetRows(nome)` (formatado) e `f.GetRows(nome, excelize.Options{RawCellValue: true})` (cru), e por célula `f.GetCellType`. Erro no `GetRows` ou nenhuma linha → `MotivoVazia`.
- Célula → `Celula`: tipo `CellTypeError` → `Erro`; `CellTypeBool` → `Bool`; cru vazio → `Vazia`; cru que faz `strconv.ParseFloat` (tipos `Number`, `Formula` ou `Unset`) → `Numero`, salvo quando o formato numérico do estilo é de data (abaixo), aí `Data` com `excelize.ExcelDateToTime(num, date1904)`; o resto → `Texto`. `Texto` da célula é sempre o valor formatado.
- Formato de data: `f.GetCellStyle` → `f.GetStyle(id)`; é data quando `NumFmt` ∈ {14–22, 27–36, 45–47, 50–58} ou quando `CustomNumFmt`, depois de tirar trechos entre aspas, colchetes e caracteres escapados com `\`, contém alguma das letras `y m d h s` (qualquer caixa).
- Nome da tabela: `copia.Identificador(arquivo sem extensão)`; se o arquivo tem ≥ 2 abas visíveis, `+ "__" + copia.Identificador(aba)`; `copia.Unico` sobre todas as tabelas da pasta.
- `copia.Gravar(destino, tabelas)` no fim; relatório sempre preenchido; erros nas mensagens exatas acima.

- [ ] **Passo 4: rodar e ver passar**

Run: `cd ctl && gofmt -l . && go vet ./... && go test ./internal/excel/`
Expected: PASS

- [ ] **Passo 5: commit**

```bash
git add ctl/go.mod ctl/go.sum ctl/internal/excel
git commit -m "cd2-ctl: leitor de planilhas .xlsx/.xlsm com excelize e relatório de abas ignoradas"
```

---

### Tarefa 4: motor `excel` no `cd2-ctl` (validação, supervisor em duas etapas, receita)

**Files:**
- Modify: `ctl/internal/dbconf/validate.go` (`Engines`, `defaultPorts`, `msgEngine`, `DefaultSchema`, `check`), `ctl/internal/dbconf/validate_test.go`
- Modify: `ctl/internal/toolbox/supervisor.go` (`Start`, `env`, `sqlitePath`), `ctl/internal/toolbox/supervisor_test.go`
- Create: `core/toolbox/excel.yaml`

**Interfaces:**
- Produces:
  - `func dbconf.EngineArquivo(engine string) bool` — verdadeiro para `excel` e `dbf`.
  - `func (s *Supervisor) Planilhas() *excel.Relatorio` — último relatório de importação Excel (nulo nos outros motores, inclusive DBF); fica guardado mesmo quando `Start` falha na importação.
  - `Start(cfg)` para `excel`/`dbf`: importa em `<sqlite>.novo`, renomeia por cima de `<sqlite>` e sobe. Falha na importação → `*StartError{Msg: err.Error()}` sem tocar na cópia anterior.
  - `sqlitePath(engine)`: `excel.sqlite` ou `dbf.sqlite` em `CacheDir`.

- [ ] **Passo 1: testes do `dbconf`**

Em `validate_test.go`, no estilo dos existentes: `excel` aceito com só a base; base vazia → `base: obrigatório`; `DefaultPort("excel") == 0`; `DefaultSchema("excel", ...) == ""`; `EngineArquivo` verdadeiro para `excel` e `dbf`, falso para `postgres`; `msgEngine` cita `excel`.

- [ ] **Passo 2: rodar e ver falhar**

Run: `cd ctl && go test ./internal/dbconf/`
Expected: FAIL

- [ ] **Passo 3: implementar no `dbconf`**

`Engines` ganha `"excel"` depois de `"dbf"`; `defaultPorts["excel"] = 0`; `msgEngine = "use postgres, oracle, mariadb, sqlserver, firebird, dbf ou excel"`; `DefaultSchema` e `check` tratam `excel` como `dbf` (trocar `c.Engine == "dbf"` por `EngineArquivo(c.Engine)`).

- [ ] **Passo 4: testes do supervisor**

Em `supervisor_test.go`, com o `faketoolbox` já usado lá:

```go
func TestStartExcelImportaEmDuasEtapasEGuardaRelatorio(t *testing.T)
// pasta com um .xlsx bom e uma aba oculta (gerar com excelize); Start(cfg excel) → sem erro; existe excel.sqlite, não existe excel.sqlite.novo; Planilhas().Tabelas == 1 e 1 ignorada; env passado ao processo tem CD2_DB_SQLITE=<cache>/excel.sqlite (reusar a conferência de ambiente de TestPassaFlagsEAmbienteSemAChave)

func TestStartExcelFalhaNaImportacaoMantemCopiaAnterior(t *testing.T)
// grava um excel.sqlite anterior com conteúdo conhecido; Start com pasta inexistente → *StartError com "Não achei a pasta das planilhas."; excel.sqlite continua com o conteúdo anterior; Planilhas() == nil (pasta inexistente não tem relatório)

func TestStartExcelTudoIgnoradoGuardaRelatorio(t *testing.T)
// pasta só com aba de texto solto → StartError; Planilhas() != nil com 1 ignorada
```

- [ ] **Passo 5: rodar e ver falhar**

Run: `cd ctl && go test ./internal/toolbox/ -run Excel`
Expected: FAIL

- [ ] **Passo 6: implementar no supervisor**

- `Start`: quando `dbconf.EngineArquivo(cfg.Engine)`: `novo := s.sqlitePath(cfg.Engine) + ".novo"`; `excel.Import(cfg.Name, novo)` ou `dbf.Import(cfg.Name, novo)`; guardar `s.planilhas = rel` (só no excel; zerar nos outros motores); em erro devolver `&StartError{Msg: err.Error()}`; em sucesso `os.Rename(novo, s.sqlitePath(cfg.Engine))` (apagando antes `-wal`/`-shm` do destino) e seguir como hoje.
- `env`: `CD2_DB_SQLITE` para os dois motores de arquivo.
- `Planilhas()` protegido por `s.mu`.

- [ ] **Passo 7: receita `core/toolbox/excel.yaml`**

Cópia de `dbf.yaml` com: comentário de cabeçalho falando de planilhas; `engine` em `db_connection_status` devolvendo `'excel'`; descrições das ferramentas trocando "arquivos .dbf"/"cópia do DBF" por "planilhas" e "cada aba é uma tabela" (`db_list_tables`: "Lista as abas das planilhas copiadas, uma tabela por aba (nome arquivo__aba quando o arquivo tem mais de uma aba)."). SQL idêntico.

- [ ] **Passo 8: rodar tudo**

Run: `cd ctl && gofmt -l . && go vet ./... && go test ./... && go test -tags sandbox ./...`
Expected: PASS

- [ ] **Passo 9: commit**

```bash
git add ctl/internal/dbconf ctl/internal/toolbox core/toolbox/excel.yaml
git commit -m "cd2-ctl: motor excel (validação, importação em duas etapas, relatório) e receita do toolbox"
```

---

### Tarefa 5: `Reload` no supervisor, `Reler` no controlador, `POST /reler` e `planilhas` no status

**Files:**
- Modify: `ctl/internal/toolbox/supervisor.go`, `ctl/internal/toolbox/supervisor_test.go`
- Modify: `ctl/internal/control/control.go` (`Runner`, `Status`, `Status()`, `Reler`), `ctl/internal/control/control_test.go` (`fakeRunner` ganha `Reload` e `Planilhas`)
- Modify: `ctl/internal/api/api.go` (`Backend`, rota `/reler`), `ctl/internal/api/api_test.go` (`fakeBackend` ganha `Reler`)

**Interfaces:**
- Consumes: `Start` em duas etapas, `Planilhas()`, `dbconf.EngineArquivo`.
- Produces:
  - `func (s *Supervisor) Reload(cfg dbconf.Config) error` — importa em `.novo`; se falhar, devolve `*StartError` e **não para** o toolbox; se der certo, `Stop`, renomeia, sobe de novo (mesmo caminho do `Start`).
  - `Runner` ganha `Reload(dbconf.Config) error` e `Planilhas() *excel.Relatorio`.
  - `Status` ganha `Planilhas *excel.Relatorio \`json:"planilhas,omitempty"\``, preenchido em `Status()` a partir de `c.run.Planilhas()`.
  - `var ErrNaoArquivo = errors.New("Esta conexão não é de arquivos.")`
  - `func (c *Controller) Reler() (Status, error)` — `ErrNaoArquivo` sem conexão ou com motor que não é de arquivo; senão `Subindo` → `Reload`; em erro com `c.run.Running()` verdadeiro, `Rodando` com `erro` (cópia antiga no ar); em erro com o toolbox parado, `rollback(cur, err)`; sucesso → `Rodando` limpo.
  - `Backend` ganha `Reler() (control.Status, error)`; rota `POST /reler` (outro método → 405 `Allow: POST`); `ErrNaoArquivo` → 409 `{"erro":"Esta conexão não é de arquivos."}`; senão `writeJSON(codeFor(st), st)`.

- [ ] **Passo 1: testes do `Reload` no supervisor**

```go
func TestReloadTrocaACopiaComToolboxDePe(t *testing.T)
// Start com pasta A (1 tabela); acrescenta um .xlsx na pasta; Reload → Running() verdadeiro; excel.sqlite tem 2 tabelas; Planilhas().Tabelas == 2

func TestReloadQueFalhaNaImportacaoNaoDerruba(t *testing.T)
// Start ok; apaga a pasta; Reload → *StartError "Não achei a pasta das planilhas."; Running() continua verdadeiro; excel.sqlite intacto (1 tabela)
```

- [ ] **Passo 2: testes do controlador**

No `control_test.go`, `fakeRunner` ganha `reloads []string`, `reloadErr error`, `planilhas *excel.Relatorio`; `Reload` registra e devolve `reloadErr` (sem mexer em `running` quando falha).

```go
func TestRelerComExcel(t *testing.T)            // Apply excel; Reler → Rodando, erro vazio, 1 reload
func TestRelerComDbf(t *testing.T)              // igual com dbf
func TestRelerFalhaNaImportacaoMantemRodando(t *testing.T) // reloadErr = &toolbox.StartError{Msg:"x"}; Reler → Rodando com Erro "x"; stops não aumentou
func TestRelerSemConexaoENaoArquivo(t *testing.T) // sem Apply → ErrNaoArquivo; Apply postgres → ErrNaoArquivo
func TestStatusLevaPlanilhas(t *testing.T)      // fakeRunner.planilhas != nil → Status().Planilhas é o mesmo ponteiro; nil nos outros
```

- [ ] **Passo 3: testes da API**

`fakeBackend` ganha `relidos int`, `relerErr error`. Testes: `POST /reler` → 200 com o status; `relerErr = control.ErrNaoArquivo` → 409 com a mensagem; `GET /reler` → 405; sem chave → 401 (o `do` com `Authorization` vazio); `planilhas` aparece no JSON de `/status` quando o fake devolve um relatório.

- [ ] **Passo 4: rodar e ver falhar**

Run: `cd ctl && go test ./internal/toolbox/ ./internal/control/ ./internal/api/`
Expected: FAIL (não compila)

- [ ] **Passo 5: implementar**

Supervisor: extrair de `Start` a parte que importa (`importar(cfg) (commit func() error, err error)`, que grava `.novo` e devolve a troca) e a parte que sobe o processo (`subir(cfg) error`). `Start` = importar, commit, subir. `Reload` = importar; se erro, devolver sem parar; `Stop`; commit; subir.

Controlador e API como nas interfaces. `Reler` usa o mesmo `c.op` do `Apply`.

- [ ] **Passo 6: rodar tudo**

Run: `cd ctl && gofmt -l . && go vet ./... && go test ./... && go test -tags sandbox ./...`
Expected: PASS

- [ ] **Passo 7: commit**

```bash
git add ctl/internal/toolbox ctl/internal/control ctl/internal/api
git commit -m "cd2-ctl: POST /reler relê a pasta (excel e dbf) sem derrubar a cópia que está no ar; planilhas no /status"
```

---

### Tarefa 6: front — motor na tela, dialeto, linha "Planilhas lidas" e abas ignoradas

**Files:**
- Modify: `web/toolbox.js` (`ENGINES`, `PLACEHOLDERS`, `defaultsFor`, `placeholdersFor`; novas `planilhasLinha`, `ignoradasLinhas`, `motorArquivo`), `tests/toolbox.test.js`
- Modify: `web/ask.js` (`DIALETO.excel`, `schemaPrefix`), `tests/ask.test.js`
- Modify: `web/index.html` (`<option>`, `syncDbForm`, `renderToolbox`, `<details id="tbIgnoradas">` dentro de `#tbBox`)

**Interfaces:**
- Produces (em `CD2Toolbox`):
  - `ENGINES.excel = { label:'Planilhas Excel', port:0 }`
  - `motorArquivo(motor) → boolean` (verdadeiro para `excel` e `dbf`)
  - `planilhasLinha(p) → string` — `"4 arquivos, 9 abas viraram tabelas"` (singular: `"1 arquivo, 1 aba virou tabela"`)
  - `ignoradasLinhas(p) → string[]` — uma por ignorada: `"vendas 2026.xlsx › Rascunho: aba oculta"`; sem aba: `"ruim.xlsx: o arquivo não abriu"`
- Produces (em `CD2Ask`): `DIALETO.excel = { nome:'Planilhas Excel (consultadas como SQLite; cada aba é uma tabela, nome arquivo__aba; pode não haver chave ligando as tabelas; LIMIT; data em texto AAAA-MM-DD)', mes: strftime como o dbf, top: LIMIT 10 }`; `schemaPrefix` devolve `''` para `excel`.

- [ ] **Passo 1: testes do `toolbox.js`**

```js
test('excel: rótulo, porta 0, schema vazio e placeholders', () => {
  assert.equal(T.engineLabel('excel'), 'Planilhas Excel');
  assert.deepEqual(T.defaultsFor('excel', '', 'C:\\Controle da loja'), { porta:0, schema:'' });
  const p = T.placeholdersFor('excel', '', '');
  assert.equal(p.base, 'ex.: C:\\Controle da loja'); assert.equal(p.host, 'ex.: nao usado'); assert.equal(p.schema, 'ex.: nao usado');
  assert.equal(T.configPayload({ motor:'excel', base:' C:\\x ', host:'', usuario:'', schema:'', senha:'' }).base, 'C:\\x');
});
test('motorArquivo, planilhasLinha e ignoradasLinhas', () => {
  assert.equal(T.motorArquivo('excel'), true); assert.equal(T.motorArquivo('dbf'), true); assert.equal(T.motorArquivo('postgres'), false);
  assert.equal(T.planilhasLinha({ arquivos:4, tabelas:9, ignoradas:[] }), '4 arquivos, 9 abas viraram tabelas');
  assert.equal(T.planilhasLinha({ arquivos:1, tabelas:1, ignoradas:[] }), '1 arquivo, 1 aba virou tabela');
  assert.deepEqual(T.ignoradasLinhas({ ignoradas:[{ arquivo:'vendas 2026.xlsx', aba:'Rascunho', motivo:'aba oculta' }, { arquivo:'ruim.xlsx', aba:'', motivo:'o arquivo não abriu' }] }),
    ['vendas 2026.xlsx › Rascunho: aba oculta', 'ruim.xlsx: o arquivo não abriu']);
});
```

Incluir `excel` no laço `for(const motor of [...])` do teste existente de `portFor`/`placeholdersFor` (linha ~208).

- [ ] **Passo 2: testes do `ask.js`**

Ao lado da linha 221: `assert.match(sys('excel'), /LIMIT 10/); assert.match(sys('excel'), /strftime/); assert.match(sys('excel'), /arquivo__aba/);` e `assert.equal(A.schemaPrefix('excel', 'x', 'u'), '')` (conferir o nome exportado em `ask.js`).

- [ ] **Passo 3: rodar e ver falhar**

Run: `node --test tests/toolbox.test.js tests/ask.test.js`
Expected: FAIL

- [ ] **Passo 4: implementar em `toolbox.js` e `ask.js`**

Exportar as três funções novas em `api`.

- [ ] **Passo 5: `index.html`**

- `<option value="excel">Planilhas Excel</option>` depois de DBF.
- `syncDbForm`: rótulo `Pasta das planilhas` para `excel`.
- Dentro de `#tbBox`, depois de `#tbHint`: `<details class="tool" id="tbIgnoradas" style="display:none"><summary></summary><ul class="inner"></ul></details>`.
- `renderToolbox`: depois das `lines(...)` do estado `rodando` **e também** quando `cs.erro` com `cs.planilhas` presente: `lines([['Planilhas lidas', CD2Toolbox.planilhasLinha(cs.planilhas)]])`; se `cs.planilhas.ignoradas.length`, mostrar `#tbIgnoradas` com `summary` = `Abas deixadas de fora (N)` e um `<li>` por linha de `ignoradasLinhas` (`textContent`, nunca `innerHTML`). Esconder o `details` em todos os outros caminhos (ele é zerado no início da função, como `logBox`).

- [ ] **Passo 6: rodar os testes e conferir na tela**

Run: `node --test`
Expected: PASS

Conferência manual: subir o app pelo lançador local com uma pasta de planilhas de teste (gerar duas com o LibreOffice ou Excel: uma simples, uma com título, aba oculta e total), escolher `Planilhas Excel`, `Salvar e conectar`; ver em Configurações a linha "Planilhas lidas" e o `details`; perguntar "quantas linhas tem cada tabela" na Conversa.

- [ ] **Passo 7: commit**

```bash
git add web/toolbox.js web/ask.js web/index.html tests/toolbox.test.js tests/ask.test.js
git commit -m "Tela: motor Planilhas Excel, dialeto para o modelo e abas deixadas de fora em Configurações"
```

---

### Tarefa 7: ↻ Atualizar com o banco relê a pasta (Excel e DBF) e reabre a sessão MCP; aviso de varejo pulado

**Files:**
- Modify: `web/index.html` (`<dialog id="atualizar">`, `syncAtualizar`, `abrirAtualizar`, `etapaAtualizar`, nova `reabrirToolbox`, `concluirEntendimento`)

**Interfaces:**
- Consumes: `POST /reler` (Tarefa 5), `CD2Toolbox.motorArquivo` (Tarefa 6), `st.ctlStatus.motor`.
- Produces: `async function reabrirToolbox()` — cria um `McpClient` novo para `CD2Toolbox.urlFor(location)`, `init`, `db_connection_status`, atualiza `st.tools`, `st.provider`, `st.dbStatus` e a pílula `#pMcp`; **não** toca em `st.catalog`, `st.catalogGen`, `st.notas*`, `st.entendimento*`, `st.suggestions`. Lança se não conectar.

- [ ] **Passo 1: a janela**

Em `<dialog id="atualizar">`, antes de `#atLendo`: `<p class="hint" id="atRelendo" hidden></p>`. `etapaAtualizar` ganha a etapa `'relendo'`: mostra `#atRelendo` com o texto `Relendo as planilhas…` (motor `excel`) ou `Relendo os arquivos .dbf…` (motor `dbf`), esconde os demais, `ok` escondido, `cancelar` **desabilitado** (o `cd2-ctl` não cancela a releitura) com texto "Cancelar".

- [ ] **Passo 2: o botão**

`syncAtualizar`: título por motor quando `CD2Toolbox.motorArquivo(st.ctlStatus && st.ctlStatus.motor)`: `excel` → `Reler as planilhas e trazer abas e colunas novas, sem perder as notas`; `dbf` → `Reler os arquivos e trazer tabelas e colunas novas, sem perder as notas`; demais motores, o título de hoje.

- [ ] **Passo 3: `abrirAtualizar`**

Logo depois de `if(!d.open) d.showModal();`, quando `CD2Toolbox.motorArquivo(motor)` e `client && client !== demo`:

1. `etapaAtualizar('relendo')`.
2. `let r = null; try{ r = await ctlRequest('POST', '/reler', null, 120000); }catch(e){}`.
3. Guardas: `if(leitura !== at.leitura || gen !== st.catalogGen || !d.open) return;`.
4. `!r` → `etapaAtualizar('erro', 'O controle do banco não respondeu. Feche e abra a IA de Bolso pelo atalho INICIAR. Nenhuma nota foi apagada.')`; `r.status !== 200 || r.data.erro` → `etapaAtualizar('erro', (r.data && r.data.erro || 'Não consegui reler a pasta.') + ' Nenhuma nota foi apagada.')`; nos dois casos `st.ctlStatus = r && r.data || st.ctlStatus; renderToolbox();` e `return`.
5. Sucesso: `st.ctlStatus = r.data; renderToolbox();` `try{ await reabrirToolbox(); }catch(e){ return etapaAtualizar('erro', 'A pasta foi relida, mas não consegui falar com o toolbox de novo. Feche e abra a IA de Bolso pelo atalho INICIAR. Nenhuma nota foi apagada.'); }` e repetir as guardas do item 3.
6. `client = st.provider` (a variável local tem que apontar para a sessão nova) e seguir para `etapaAtualizar('lendo')` e o `db_catalog` de hoje.

`reabrirToolbox` reaproveita o trecho `mode === 'toolbox'` de `conectarModo` sem `exigirBanco` e sem `loadCatalog`: extrair desse trecho uma função `sessaoToolbox()` que devolve `{ c, statusTxt }`, usada pelos dois.

- [ ] **Passo 4: aviso de varejo pulado para planilhas**

Em `concluirEntendimento`, antes de `verificarVarejo(cat, doc)`: se `st.ctlStatus && st.ctlStatus.motor === 'excel'`, gravar em `doc.varejo` um veredito final que não dispara o aviso (ler `verificarVarejo` e `entenderNovas` para o formato: o campo `verificado:true` é o que torna o veredito final; o campo que diz "é varejo" fica verdadeiro, para o texto `TEXTO_VAREJO` nunca sair). Comentário de uma linha dizendo o porquê (planilha de controle não é ERP).

- [ ] **Passo 5: testes automáticos e conferência manual**

Run: `node --test`
Expected: PASS (nada quebrou).

Conferência manual, com o app rodando numa pasta de planilhas:

1. Conhecimento → ↻ Atualizar com o banco: aparece "Relendo as planilhas…", depois "O entendimento já está igual ao banco."
2. Acrescentar uma aba nova num arquivo (salvar no Excel/LibreOffice) → Atualizar: resumo com 1 tabela nova; confirmar; a nota de uma tabela antiga continua lá; a faixa descreve só a nova.
3. Renomear a pasta → Atualizar: erro "Não achei a pasta das planilhas. Nenhuma nota foi apagada."; a Conversa continua respondendo com a cópia antiga.
4. Clicar fora da janela durante "Relendo…" (ela não fecha com Esc nessa etapa porque `cancelar` está desabilitado; testar fechando com a tecla Esc do `dialog` se o navegador permitir): reabrir a janela recomeça do zero, sem erro no console.
5. Com o DBF da pasta de testes (`tests/ctl`, se houver `.dbf`; senão gerar um com o ajudante do `import_test.go`): o mesmo botão mostra "Relendo os arquivos .dbf…".
6. Com Excel, ao fim do entendimento, **não** aparece o aviso "não parece ser de um ERP de varejo".

- [ ] **Passo 6: commit**

```bash
git add web/index.html
git commit -m "Tela: Atualizar com o banco relê a pasta nas planilhas e no DBF, reabrindo a sessão sem perder notas; sem aviso de varejo para planilhas"
```

---

### Tarefa 8: README, site e versão 0.21.0

**Files:**
- Modify: `README.md` (seção 3: parágrafo da conexão, tabela de motores, parágrafo do Atualizar), `docs/index.html` (card "Os bancos mais comuns"), `VERSION`, `web/index.html` (`?v=`), `web/mcp.js` (`CLIENT_INFO.version`)
- Test: `tests/versao.test.js` (já existe)

- [ ] **Passo 1: README**

- Parágrafo da conexão (linha 131): acrescentar "pasta das planilhas `.xlsx` no Planilhas Excel" à lista entre parênteses.
- Tabela de motores: linha nova depois do DBF: `| Planilhas Excel | não usa | não usa | sem servidor: no campo da base vai a pasta com os arquivos .xlsx/.xlsm; cada aba vira uma tabela (nome arquivo__aba quando o arquivo tem mais de uma aba); a leitura copia e não altera a planilha |`.
- Parágrafo novo logo abaixo da tabela, em até seis linhas: o que entra (cabeçalho procurado nas 30 primeiras linhas, título em cima ignorado, linha de total e abas ocultas fora, coluna sem nome vira `Coluna_D`), o que fica de fora (`.xls` antigo e `.csv`: salvar como `.xlsx`), onde ver as abas ignoradas (Configurações), e que no Excel e no DBF o botão **↻ Atualizar com o banco** da guia Conhecimento relê a pasta antes de comparar.

- [ ] **Passo 2: site**

Em `docs/index.html`, card "Os bancos mais comuns": `... Firebird, arquivos DBF e planilhas do Excel (.xlsx), para quem ainda não tem sistema. ...`. Sem print novo.

- [ ] **Passo 3: versão**

`VERSION` → `0.21.0`; em `web/index.html` trocar todos os `?v=0.20.1` por `?v=0.21.0`; em `web/mcp.js`, `version:'0.21.0'`.

- [ ] **Passo 4: rodar tudo**

Run: `node --test && cd ctl && gofmt -l . && go vet ./... && go test ./... && go test -tags sandbox ./...`
Expected: PASS, `gofmt` sem saída.

- [ ] **Passo 5: commit**

```bash
git add README.md docs/index.html VERSION web/index.html web/mcp.js
git commit -m "README e site: Planilhas Excel como opção de banco; versão 0.21.0"
```
