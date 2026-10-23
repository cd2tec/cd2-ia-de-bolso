package excel

import (
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/copia"
)

func txt(s string) Celula { return Celula{Tipo: Texto, Texto: s} }
func num(f float64) Celula {
	return Celula{Tipo: Numero, Num: f, Texto: strconv.FormatFloat(f, 'f', -1, 64)}
}
func data(ano int, mes time.Month, dia int) Celula {
	d := time.Date(ano, mes, dia, 0, 0, 0, 0, time.UTC)
	return Celula{Tipo: Data, Data: d, Texto: d.Format("02/01/2006")}
}
func vazia() Celula { return Celula{} }

func col(nome, tipo string) copia.Coluna { return copia.Coluna{Nome: nome, Tipo: tipo} }

func esperaColunas(t *testing.T, got []copia.Coluna, want ...copia.Coluna) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("colunas: %v != %v", got, want)
	}
}

func TestCabecalhoNaPrimeiraLinha(t *testing.T) {
	grade := [][]Celula{
		{txt("Produto"), txt("Qtde"), txt("Data")},
		{txt("Arroz"), num(2), data(2026, 9, 1)},
		{txt("Feijão"), num(1.5), data(2026, 9, 2)},
	}
	cols, linhas, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	esperaColunas(t, cols, col("Produto", "TEXT"), col("Qtde", "NUMERIC"), col("Data", "DATE"))
	if !reflect.DeepEqual(linhas[0], []any{"Arroz", 2.0, "2026-09-01"}) || len(linhas) != 2 {
		t.Fatalf("linhas: %v", linhas)
	}
}

func TestPlanilhaViva(t *testing.T) {
	grade := [][]Celula{
		{txt("Controle de Vendas 2026")},
		{},
		{txt("Data"), vazia(), txt("Valor")},
		{data(2026, 9, 1), txt("Centro"), num(10)},
		{},
		{data(2026, 9, 2), txt("Bairro"), num(20)},
		{txt("Total"), vazia(), num(30)},
	}
	cols, linhas, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	esperaColunas(t, cols, col("Data", "DATE"), col("Coluna_B", "TEXT"), col("Valor", "INTEGER"))
	want := [][]any{{"2026-09-01", "Centro", int64(10)}, {"2026-09-02", "Bairro", int64(20)}}
	if !reflect.DeepEqual(linhas, want) {
		t.Fatalf("linhas: %v", linhas)
	}
}

func TestTituloMescladoLargoECabecalhoNaLinha2(t *testing.T) {
	grade := [][]Celula{
		{txt("Relatório"), vazia(), vazia()},
		{txt("Nome"), txt("Qtde"), txt("Preço (R$)")},
		{txt("a"), num(1), num(2.5)},
	}
	cols, linhas, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	esperaColunas(t, cols, col("Nome", "TEXT"), col("Qtde", "INTEGER"), col("Preco_R", "NUMERIC"))
	if len(linhas) != 1 {
		t.Fatalf("linhas: %v", linhas)
	}
}

// Ano no cabeçalho é comum ([Produto, 2024, 2025]): para candidatar a linha, um número que parece ano
// conta como texto; senão só "Produto" seria texto e a linha seria rejeitada.
func TestCabecalhoComAnoNasColunas(t *testing.T) {
	grade := [][]Celula{
		{txt("Produto"), num(2024), num(2025)},
		{txt("Arroz"), num(10), num(12)},
	}
	cols, _, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	esperaColunas(t, cols, col("Produto", "TEXT"), col("T_2024", "INTEGER"), col("T_2025", "INTEGER"))
}

func TestSoUmaCelulaDeTextoNaoECabecalho(t *testing.T) {
	grade := [][]Celula{
		{txt("Vendedor"), num(100), num(200)},
		{txt("Ana"), num(1), num(2)},
	}
	if _, _, motivo := analisarAba(grade); motivo != MotivoSemCabecalho {
		t.Fatalf("motivo: %q", motivo)
	}
}

func TestColunaMistaViraTexto(t *testing.T) {
	// O cabeçalho precisa de 2 células não vazias; por isso a coluna "Id" acompanha a "Obs".
	grade := [][]Celula{
		{txt("Id"), txt("Obs")},
		{txt("a"), num(10)},
		{txt("b"), txt("dez")},
	}
	cols, linhas, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	esperaColunas(t, cols, col("Id", "TEXT"), col("Obs", "TEXT"))
	if !reflect.DeepEqual(linhas, [][]any{{"a", "10"}, {"b", "dez"}}) {
		t.Fatalf("linhas: %v", linhas)
	}
}

func TestInteiroNumericoBoolErro(t *testing.T) {
	sim := Celula{Tipo: Bool, Texto: "TRUE", Num: 1}
	nao := Celula{Tipo: Bool, Texto: "FALSE"}
	erro := Celula{Tipo: Erro, Texto: "#DIV/0!"}
	grade := [][]Celula{
		{txt("Inteira"), txt("Quebrada"), txt("Flag"), txt("Falha")},
		{num(1), num(1), sim, num(5)},
		{num(2), num(1.5), nao, erro},
		{num(3), vazia(), sim, num(7)},
	}
	cols, linhas, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	esperaColunas(t, cols, col("Inteira", "INTEGER"), col("Quebrada", "NUMERIC"),
		col("Flag", "INTEGER"), col("Falha", "INTEGER"))
	want := [][]any{
		{int64(1), 1.0, int64(1), int64(5)},
		{int64(2), 1.5, int64(0), nil},
		{int64(3), nil, int64(1), int64(7)},
	}
	if !reflect.DeepEqual(linhas, want) {
		t.Fatalf("linhas: %v", linhas)
	}
}

func TestDataComHora(t *testing.T) {
	d := time.Date(2026, 9, 1, 14, 30, 5, 0, time.UTC)
	grade := [][]Celula{
		{txt("Quando"), txt("Obs")},
		{Celula{Tipo: Data, Data: d, Texto: "x"}, txt("a")},
	}
	_, linhas, _ := analisarAba(grade)
	if linhas[0][0] != "2026-09-01 14:30:05" {
		t.Fatalf("linhas: %v", linhas)
	}
}

// Regra escolhida: sai a linha cuja primeira célula de texto, sem acento e em minúsculas, COMEÇA com
// "total" ou "subtotal". Por isso "Totalmente ok" também sai.
func TestSubtotalComAcentoSai(t *testing.T) {
	grade := [][]Celula{
		{txt("Item"), txt("Valor")},
		{txt("a"), num(1)},
		{txt("SubTotal geral"), num(1)},
		{txt("TOTAL"), num(1)},
		{txt("Totalmente ok"), num(1)},
		{txt("Sübtotál"), num(1)},
		{txt("b"), num(2)},
	}
	_, linhas, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	// "Sübtotál" vira "subtotal" sem acento e também sai.
	if len(linhas) != 2 || linhas[0][0] != "a" || linhas[1][0] != "b" {
		t.Fatalf("linhas: %v", linhas)
	}
}

func TestAbaVaziaESemCandidata(t *testing.T) {
	if _, _, motivo := analisarAba(nil); motivo != MotivoVazia {
		t.Fatalf("motivo: %q", motivo)
	}
	var grade [][]Celula
	for i := 0; i < 30; i++ {
		grade = append(grade, []Celula{num(float64(i)), num(1)})
	}
	if _, _, motivo := analisarAba(grade); motivo != MotivoSemCabecalho {
		t.Fatalf("motivo: %q", motivo)
	}
}

func TestCabecalhoDepoisDaLinha30NaoConta(t *testing.T) {
	var grade [][]Celula
	for i := 0; i < 30; i++ {
		grade = append(grade, []Celula{num(float64(i)), num(1)})
	}
	grade = append(grade, []Celula{txt("Nome"), txt("Qtde")}, []Celula{txt("a"), num(1)})
	if _, _, motivo := analisarAba(grade); motivo != MotivoSemCabecalho {
		t.Fatalf("motivo: %q", motivo)
	}
}

func TestLinhaCurtaECelulaForaDaLargura(t *testing.T) {
	grade := [][]Celula{
		{txt("A"), txt("B")},
		{txt("x")},
		{txt("y"), num(1), txt("fora da largura")},
	}
	cols, linhas, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	if len(cols) != 2 || !reflect.DeepEqual(linhas, [][]any{{"x", nil}, {"y", int64(1)}}) {
		t.Fatalf("cols %v linhas %v", cols, linhas)
	}
}

func TestLetraDaColunaEhAbsoluta(t *testing.T) {
	// A largura começa na coluna C; o nome da célula vazia usa a letra real da coluna.
	grade := [][]Celula{
		{vazia(), vazia(), txt("Nome"), vazia(), txt("Qtde")},
		{vazia(), vazia(), txt("a"), txt("b"), num(1)},
	}
	cols, _, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	esperaColunas(t, cols, col("Nome", "TEXT"), col("Coluna_D", "TEXT"), col("Qtde", "INTEGER"))
}

func TestLetrasDeColuna(t *testing.T) {
	casos := map[int]string{1: "A", 26: "Z", 27: "AA", 28: "AB", 52: "AZ", 53: "BA", 702: "ZZ", 703: "AAA"}
	for n, want := range casos {
		if got := letraDaColuna(n); got != want {
			t.Errorf("%d: %q != %q", n, got, want)
		}
	}
}
