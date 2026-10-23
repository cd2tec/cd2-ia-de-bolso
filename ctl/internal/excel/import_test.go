package excel

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

// novaPlanilha cria um .xlsx em caminho, deixa preencher e salva. A aba inicial chama "Sheet1".
func novaPlanilha(t *testing.T, caminho string, preencher func(f *excelize.File)) {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	preencher(f)
	if err := f.SaveAs(caminho); err != nil {
		t.Fatal(err)
	}
}

// deve falha o teste quando a chamada à excelize no preenchimento dá erro.
func deve(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func estiloData(t *testing.T, f *excelize.File) int {
	t.Helper()
	id, err := f.NewStyle(&excelize.Style{NumFmt: 14})
	deve(t, err)
	return id
}

// planilhaSimples grava uma aba com cabeçalho e uma linha, para servir de arquivo bom ao lado de outro.
func planilhaSimples(t *testing.T, caminho string) {
	t.Helper()
	novaPlanilha(t, caminho, func(f *excelize.File) {
		deve(t, f.SetSheetRow("Sheet1", "A1", &[]any{"Produto", "Qtd"}))
		deve(t, f.SetSheetRow("Sheet1", "A2", &[]any{"Arroz", 3}))
	})
}

func abrir(t *testing.T, destino string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", destino)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func tabelasDe(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var nomes []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		nomes = append(nomes, n)
	}
	return nomes
}

func tiposDe(t *testing.T, db *sql.DB, tabela string) map[string]string {
	t.Helper()
	rows, err := db.Query(`SELECT name, type FROM pragma_table_info(?)`, tabela)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	tipos := map[string]string{}
	for rows.Next() {
		var n, tp string
		if err := rows.Scan(&n, &tp); err != nil {
			t.Fatal(err)
		}
		tipos[n] = tp
	}
	return tipos
}

func TestImportBemComportada(t *testing.T) {
	dir := t.TempDir()
	novaPlanilha(t, filepath.Join(dir, "clientes.xlsx"), func(f *excelize.File) {
		deve(t, f.SetSheetName("Sheet1", "Plan1"))
		deve(t, f.SetSheetRow("Plan1", "A1", &[]any{"Nome", "Limite", "Desde"}))
		deve(t, f.SetSheetRow("Plan1", "A2", &[]any{"Ana", 1500.5, time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)}))
		deve(t, f.SetSheetRow("Plan1", "A3", &[]any{"Bruno", 800, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)}))
		deve(t, f.SetCellStyle("Plan1", "C2", "C3", estiloData(t, f)))
	})
	destino := filepath.Join(dir, "out", "planilhas.sqlite")
	rel, err := Import(dir, destino)
	if err != nil {
		t.Fatal(err)
	}
	if rel.Arquivos != 1 || rel.Tabelas != 1 || rel.Ignoradas == nil || len(rel.Ignoradas) != 0 {
		t.Fatalf("relatório: %+v", rel)
	}
	if rel.LidoEm.IsZero() {
		t.Fatal("LidoEm vazio")
	}
	js, err := json.Marshal(rel)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), `"ignoradas":[]`) {
		t.Fatalf("ignoradas no JSON: %s", js)
	}
	db := abrir(t, destino)
	if got := tabelasDe(t, db); !reflect.DeepEqual(got, []string{"clientes"}) {
		t.Fatalf("tabelas: %v", got)
	}
	want := map[string]string{"Nome": "TEXT", "Limite": "NUMERIC", "Desde": "DATE"}
	if got := tiposDe(t, db, "clientes"); !reflect.DeepEqual(got, want) {
		t.Fatalf("tipos: %v", got)
	}
	// O driver devolve coluna DATE como time.Time; o CAST mostra o texto que de fato está gravado.
	var desde, tipo string
	if err := db.QueryRow(`SELECT CAST(Desde AS TEXT), typeof(Desde) FROM clientes WHERE Nome = 'Ana'`).Scan(&desde, &tipo); err != nil {
		t.Fatal(err)
	}
	if desde != "2026-01-15" || tipo != "text" {
		t.Fatalf("Desde: %q (%s)", desde, tipo)
	}
}

func TestImportVivaComDuasAbasEUmaOculta(t *testing.T) {
	dir := t.TempDir()
	novaPlanilha(t, filepath.Join(dir, "vendas 2026.xlsx"), func(f *excelize.File) {
		deve(t, f.SetSheetName("Sheet1", "Setembro"))
		deve(t, f.SetCellValue("Setembro", "A1", "Vendas de setembro"))
		deve(t, f.MergeCell("Setembro", "A1", "C1"))
		deve(t, f.SetSheetRow("Setembro", "A3", &[]any{"Loja", nil, "Valor"}))
		deve(t, f.SetSheetRow("Setembro", "A4", &[]any{"Centro", "balcão", 100}))
		deve(t, f.SetSheetRow("Setembro", "A5", &[]any{"Norte", "entrega", 200}))
		deve(t, f.SetSheetRow("Setembro", "A7", &[]any{"Total", nil, 300}))

		_, err := f.NewSheet("Outubro")
		deve(t, err)
		deve(t, f.SetSheetRow("Outubro", "A1", &[]any{"Loja", "Valor"}))
		deve(t, f.SetSheetRow("Outubro", "A2", &[]any{"Sul", 50}))

		_, err = f.NewSheet("Rascunho")
		deve(t, err)
		deve(t, f.SetSheetRow("Rascunho", "A1", &[]any{"Ideia", "Prazo"}))
		deve(t, f.SetSheetRow("Rascunho", "A2", &[]any{"abrir loja", "2027"}))
		deve(t, f.SetSheetVisible("Rascunho", false))
	})
	destino := filepath.Join(dir, "out", "planilhas.sqlite")
	rel, err := Import(dir, destino)
	if err != nil {
		t.Fatal(err)
	}
	wantIgn := []Ignorada{{Arquivo: "vendas 2026.xlsx", Aba: "Rascunho", Motivo: MotivoOculta}}
	if !reflect.DeepEqual(rel.Ignoradas, wantIgn) {
		t.Fatalf("ignoradas: %+v", rel.Ignoradas)
	}
	if rel.Tabelas != 2 {
		t.Fatalf("tabelas no relatório: %d", rel.Tabelas)
	}
	db := abrir(t, destino)
	if got := tabelasDe(t, db); !reflect.DeepEqual(got, []string{"vendas_2026__Outubro", "vendas_2026__Setembro"}) {
		t.Fatalf("tabelas: %v", got)
	}
	if _, ok := tiposDe(t, db, "vendas_2026__Setembro")["Coluna_B"]; !ok {
		t.Fatal("Coluna_B não veio")
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM vendas_2026__Setembro`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("linhas de Setembro: %d", n)
	}
}

func TestImportAbaSoComTextoSolto(t *testing.T) {
	dir := t.TempDir()
	novaPlanilha(t, filepath.Join(dir, "controle.xlsx"), func(f *excelize.File) {
		deve(t, f.SetSheetName("Sheet1", "Notas"))
		deve(t, f.SetCellValue("Notas", "A1", "anotações"))
		deve(t, f.SetCellValue("Notas", "A5", "ligar para o João"))
		_, err := f.NewSheet("Dados")
		deve(t, err)
		deve(t, f.SetSheetRow("Dados", "A1", &[]any{"Produto", "Qtd"}))
		deve(t, f.SetSheetRow("Dados", "A2", &[]any{"Feijão", 2}))
	})
	rel, err := Import(dir, filepath.Join(dir, "out", "planilhas.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	wantIgn := []Ignorada{{Arquivo: "controle.xlsx", Aba: "Notas", Motivo: MotivoSemCabecalho}}
	if !reflect.DeepEqual(rel.Ignoradas, wantIgn) {
		t.Fatalf("ignoradas: %+v", rel.Ignoradas)
	}
	if rel.Tabelas != 1 {
		t.Fatalf("tabelas: %d", rel.Tabelas)
	}
}

func TestImportColunaDeDataComTextoNoMeio(t *testing.T) {
	dir := t.TempDir()
	novaPlanilha(t, filepath.Join(dir, "agenda.xlsx"), func(f *excelize.File) {
		deve(t, f.SetSheetRow("Sheet1", "A1", &[]any{"Evento", "Quando"}))
		deve(t, f.SetSheetRow("Sheet1", "A2", &[]any{"Inventário", time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)}))
		deve(t, f.SetSheetRow("Sheet1", "A3", &[]any{"Balanço", time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)}))
		deve(t, f.SetSheetRow("Sheet1", "A4", &[]any{"Feira", "a confirmar"}))
		deve(t, f.SetCellStyle("Sheet1", "B2", "B3", estiloData(t, f)))
	})
	destino := filepath.Join(dir, "out", "planilhas.sqlite")
	if _, err := Import(dir, destino); err != nil {
		t.Fatal(err)
	}
	db := abrir(t, destino)
	if tp := tiposDe(t, db, "agenda")["Quando"]; tp != "TEXT" {
		t.Fatalf("tipo de Quando: %s", tp)
	}
	soNumero := regexp.MustCompile(`^\d+(\.\d+)?$`)
	for _, evento := range []string{"Inventário", "Balanço"} {
		var quando string
		if err := db.QueryRow(`SELECT Quando FROM agenda WHERE Evento = ?`, evento).Scan(&quando); err != nil {
			t.Fatal(err)
		}
		if quando == "" || soNumero.MatchString(quando) {
			t.Fatalf("data de %s veio como %q, não como o texto que o Excel mostra", evento, quando)
		}
	}
}

func TestImportFormulaSemValorGravadoVemVazia(t *testing.T) {
	dir := t.TempDir()
	novaPlanilha(t, filepath.Join(dir, "pedido.xlsx"), func(f *excelize.File) {
		deve(t, f.SetSheetRow("Sheet1", "A1", &[]any{"Qtd", "Preco", "Total"}))
		deve(t, f.SetSheetRow("Sheet1", "A2", &[]any{2, 3}))
		deve(t, f.SetCellFormula("Sheet1", "C2", "A2*B2"))
	})
	destino := filepath.Join(dir, "out", "planilhas.sqlite")
	if _, err := Import(dir, destino); err != nil {
		t.Fatal(err)
	}
	db := abrir(t, destino)
	if tp := tiposDe(t, db, "pedido")["Total"]; tp != "TEXT" {
		t.Fatalf("tipo de Total: %s", tp)
	}
	var total sql.NullString
	if err := db.QueryRow(`SELECT Total FROM pedido`).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total.Valid {
		t.Fatalf("Total devia vir nulo, veio %q", total.String)
	}
}

func TestImportIgnoraTravaXlsEDicaDeSalvarComoXlsx(t *testing.T) {
	dir := t.TempDir()
	for nome, conteudo := range map[string]string{
		"~$vendas.xlsx": "trava do Excel",
		"antigo.xls":    "binário antigo",
		"dados.csv":     "nome;valor\nAna;10\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, nome), []byte(conteudo), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rel, err := Import(dir, filepath.Join(dir, "out", "planilhas.sqlite"))
	if err == nil {
		t.Fatal("pasta sem .xlsx passou")
	}
	if !strings.Contains(err.Error(), "Não achei planilha .xlsx ou .xlsm nessa pasta.") || !strings.Contains(err.Error(), "salve como .xlsx") {
		t.Fatalf("mensagem: %q", err)
	}
	if rel == nil {
		t.Fatal("relatório nulo")
	}
	if len(rel.Ignoradas) != 0 {
		t.Fatalf("a trava não devia aparecer: %+v", rel.Ignoradas)
	}
}

func TestImportArquivoCorrompidoNaoDerruba(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ruim.xlsx"), []byte("isto não é um zip\x00\x01\x02"), 0o600); err != nil {
		t.Fatal(err)
	}
	planilhaSimples(t, filepath.Join(dir, "bom.xlsx"))
	rel, err := Import(dir, filepath.Join(dir, "out", "planilhas.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	wantIgn := []Ignorada{{Arquivo: "ruim.xlsx", Aba: "", Motivo: MotivoNaoAbriu}}
	if !reflect.DeepEqual(rel.Ignoradas, wantIgn) {
		t.Fatalf("ignoradas: %+v", rel.Ignoradas)
	}
	if rel.Tabelas != 1 {
		t.Fatalf("tabelas: %d", rel.Tabelas)
	}
}

func TestImportProtegidoPorSenha(t *testing.T) {
	dir := t.TempDir()
	f := excelize.NewFile()
	deve(t, f.SetSheetRow("Sheet1", "A1", &[]any{"Produto", "Qtd"}))
	deve(t, f.SetSheetRow("Sheet1", "A2", &[]any{"Arroz", 3}))
	deve(t, f.SaveAs(filepath.Join(dir, "segredo.xlsx"), excelize.Options{Password: "x"}))
	deve(t, f.Close())
	planilhaSimples(t, filepath.Join(dir, "aberto.xlsx"))
	rel, err := Import(dir, filepath.Join(dir, "out", "planilhas.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	wantIgn := []Ignorada{{Arquivo: "segredo.xlsx", Aba: "", Motivo: MotivoSenha}}
	if !reflect.DeepEqual(rel.Ignoradas, wantIgn) {
		t.Fatalf("ignoradas: %+v", rel.Ignoradas)
	}
}

func TestImportTudoIgnoradoDevolveRelatorio(t *testing.T) {
	dir := t.TempDir()
	novaPlanilha(t, filepath.Join(dir, "lembretes.xlsx"), func(f *excelize.File) {
		deve(t, f.SetCellValue("Sheet1", "A1", "anotações"))
		deve(t, f.SetCellValue("Sheet1", "A5", "ligar para o João"))
	})
	rel, err := Import(dir, filepath.Join(dir, "out", "planilhas.sqlite"))
	if err == nil || err.Error() != "Nenhuma aba das planilhas dessa pasta tem um cabeçalho que eu reconheça." {
		t.Fatalf("erro: %v", err)
	}
	if rel == nil {
		t.Fatal("relatório nulo")
	}
	if len(rel.Ignoradas) != 1 {
		t.Fatalf("ignoradas: %+v", rel.Ignoradas)
	}
}

func TestImportNomesIguaisEmCaixaDiferente(t *testing.T) {
	dir := t.TempDir()
	planilhaSimples(t, filepath.Join(dir, "Vendas.xlsx"))
	planilhaSimples(t, filepath.Join(dir, "vendas.xlsx"))
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entradas) != 2 {
		t.Skip("o sistema de arquivos não distingue caixa no nome")
	}
	destino := filepath.Join(dir, "out", "planilhas.sqlite")
	if _, err := Import(dir, destino); err != nil {
		t.Fatal(err)
	}
	if got := tabelasDe(t, abrir(t, destino)); !reflect.DeepEqual(got, []string{"Vendas", "vendas_2"}) {
		t.Fatalf("tabelas: %v", got)
	}
}

func TestImportPastaInexistenteENaoPasta(t *testing.T) {
	dir := t.TempDir()
	rel, err := Import(filepath.Join(dir, "nao-existe"), filepath.Join(dir, "x.sqlite"))
	if rel != nil || err == nil || err.Error() != "Não achei a pasta das planilhas." {
		t.Fatalf("pasta inexistente: rel=%v err=%v", rel, err)
	}
	arquivo := filepath.Join(dir, "vendas.xlsx")
	planilhaSimples(t, arquivo)
	rel, err = Import(arquivo, filepath.Join(dir, "x.sqlite"))
	if rel != nil || err == nil || err.Error() != "A base das planilhas é uma pasta com os arquivos .xlsx, não um arquivo." {
		t.Fatalf("arquivo no lugar da pasta: rel=%v err=%v", rel, err)
	}
}

func TestFormatoDeData(t *testing.T) {
	casos := []struct {
		numFmt int
		custom string
		quer   bool
	}{
		{14, "", true},
		{0, "", false},
		{0, "dd/mm/yyyy", true},
		{0, "#,##0.00", false},
		{0, `0.00 "m"`, false},
		{0, "[h]:mm:ss", true},
		{0, "R$ #,##0.00", false},
	}
	for _, c := range casos {
		if got := formatoDeData(c.numFmt, c.custom); got != c.quer {
			t.Errorf("formatoDeData(%d, %q) = %v, quero %v", c.numFmt, c.custom, got, c.quer)
		}
	}
}
