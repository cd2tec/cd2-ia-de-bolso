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
// conta como texto (idem Data, no teste abaixo); senão só "Produto" seria texto e a linha seria rejeitada.
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

// Controle diário por data: [Vendedor, 01/09, 02/09], com as datas guardadas como Data pelo Excel.
func TestCabecalhoComDatasNasColunas(t *testing.T) {
	d1 := Celula{Tipo: Data, Data: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Texto: "01/09"}
	d2 := Celula{Tipo: Data, Data: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), Texto: "02/09"}
	grade := [][]Celula{
		{txt("Vendedor"), d1, d2},
		{txt("Ana"), num(10), num(20)},
	}
	cols, linhas, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	esperaColunas(t, cols, col("Vendedor", "TEXT"), col("T_01_09", "INTEGER"), col("T_02_09", "INTEGER"))
	if len(linhas) != 1 {
		t.Fatalf("linhas: %v", linhas)
	}
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

// Em coluna TEXT o número vai na forma canônica, não no texto formatado da excelize, que sai americano
// ("R$ 1,500.50") e quebra SUM(CAST(...)). Data vai em ISO e Bool em "1"/"0".
func TestNumeroEmColunaTextoVaiCanonico(t *testing.T) {
	moeda := Celula{Tipo: Numero, Num: 1500.5, Texto: "R$ 1,500.50"}
	sim := Celula{Tipo: Bool, Texto: "TRUE", Num: 1}
	nao := Celula{Tipo: Bool, Texto: "FALSE"}
	grade := [][]Celula{
		{txt("Item"), txt("Valor"), txt("Obs")},
		{txt("a"), moeda, sim},
		{txt("b"), txt("-"), nao},
		{txt("c"), num(2500.25), data(2026, 9, 1)},
	}
	cols, linhas, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	esperaColunas(t, cols, col("Item", "TEXT"), col("Valor", "TEXT"), col("Obs", "TEXT"))
	want := [][]any{{"a", "1500.5", "1"}, {"b", "-", "0"}, {"c", "2500.25", "2026-09-01"}}
	if !reflect.DeepEqual(linhas, want) {
		t.Fatalf("linhas: %v", linhas)
	}
}

// Título de duas células ([Controle de Vendas, 2026]) também é candidata, mas a linha logo abaixo é
// candidata mais larga: o cabeçalho é ela.
func TestTituloDeDuasCelulasNaoViraCabecalho(t *testing.T) {
	grade := [][]Celula{
		{txt("Controle de Vendas"), num(2026)},
		{txt("Data"), txt("Produto"), txt("Qtd"), txt("Valor")},
		{data(2026, 9, 1), txt("Arroz"), num(2), num(10.5)},
	}
	cols, linhas, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	esperaColunas(t, cols, col("Data", "DATE"), col("Produto", "TEXT"), col("Qtd", "INTEGER"), col("Valor", "NUMERIC"))
	if !reflect.DeepEqual(linhas, [][]any{{"2026-09-01", "Arroz", int64(2), 10.5}}) {
		t.Fatalf("linhas: %v", linhas)
	}
}

// A troca só vale quando a linha de baixo também é candidata; em [Produto, 2024, 2025] / [Arroz, 10, 12]
// a de baixo é dado e o cabeçalho continua o de cima (ver TestCabecalhoComAnoNasColunas).
func TestLinhaDeBaixoMaisLargaMasNaoCandidata(t *testing.T) {
	grade := [][]Celula{
		{txt("Produto"), num(2024)},
		{txt("Arroz"), num(10), num(12)},
	}
	cols, linhas, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	esperaColunas(t, cols, col("Produto", "TEXT"), col("T_2024", "INTEGER"))
	if len(linhas) != 1 {
		t.Fatalf("linhas: %v", linhas)
	}
}

// Fórmula arrastada até o fim da folha dá #N/A nas linhas sem dado: linha só com Erro não é registro.
func TestLinhaSoComErroSai(t *testing.T) {
	na := Celula{Tipo: Erro, Texto: "#N/A"}
	grade := [][]Celula{
		{txt("Produto"), txt("Preço")},
		{txt("Arroz"), num(10)},
		{na, na},
		{vazia(), na},
	}
	_, linhas, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	if !reflect.DeepEqual(linhas, [][]any{{"Arroz", int64(10)}}) {
		t.Fatalf("linhas: %v", linhas)
	}
}

func TestAbaSoComCabecalho(t *testing.T) {
	grade := [][]Celula{
		{txt("Cadastro de clientes")},
		{},
		{txt("Nome"), txt("Telefone"), txt("Cidade")},
	}
	if _, _, motivo := analisarAba(grade); motivo != MotivoSoCabecalho {
		t.Fatalf("motivo: %q", motivo)
	}
	// Texto solto, sem linha com cara de cabeçalho, continua sendo "sem cabeçalho".
	if _, _, motivo := analisarAba([][]Celula{{txt("anotações")}, {txt("ligar")}}); motivo != MotivoSemCabecalho {
		t.Fatalf("motivo do texto solto: %q", motivo)
	}
}

// "#" vira identificador vazio ("T_") e "#REF!" é célula Erro: os dois caem no nome pela letra.
func TestCabecalhoSemNomeUtilViraColunaPelaLetra(t *testing.T) {
	ref := Celula{Tipo: Erro, Texto: "#REF!"}
	grade := [][]Celula{
		{txt("#"), txt("Produto"), ref, txt("2024")},
		{num(1), txt("Arroz"), num(3), num(4)},
	}
	cols, _, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	esperaColunas(t, cols, col("Coluna_A", "INTEGER"), col("Produto", "TEXT"), col("Coluna_C", "INTEGER"), col("T_2024", "INTEGER"))
}

// Primeira linha de dados mais larga que o cabeçalho (coluna de código sem título na frente, ou uma
// anotação solta na cauda) não troca o cabeçalho: só título de até 2 células deixa a escolha descer.
func TestDadoMaisLargoNaoTrocaOCabecalho(t *testing.T) {
	t.Run("coluna de codigo sem titulo", func(t *testing.T) {
		grade := [][]Celula{
			{vazia(), txt("Nome"), txt("Telefone"), txt("Cidade")},
			{num(1), txt("João"), txt("9999-1234"), txt("Curitiba")},
			{num(2), txt("Maria"), txt("9888-0000"), txt("Londrina")},
		}
		cols, linhas, motivo := analisarAba(grade)
		if motivo != "" {
			t.Fatal(motivo)
		}
		esperaColunas(t, cols, col("Coluna_A", "INTEGER"), col("Nome", "TEXT"), col("Telefone", "TEXT"), col("Cidade", "TEXT"))
		if len(linhas) != 2 {
			t.Fatalf("linhas: %v", linhas)
		}
	})
	t.Run("anotacao na cauda da primeira linha", func(t *testing.T) {
		grade := [][]Celula{
			{txt("Data"), txt("Produto"), txt("Qtd")},
			{data(2026, 9, 1), txt("Arroz"), num(2), txt("pago")},
			{data(2026, 9, 2), txt("Feijão"), num(1)},
		}
		cols, linhas, motivo := analisarAba(grade)
		if motivo != "" {
			t.Fatal(motivo)
		}
		esperaColunas(t, cols, col("Data", "DATE"), col("Produto", "TEXT"), col("Qtd", "INTEGER"))
		if len(linhas) != 2 {
			t.Fatalf("linhas: %v", linhas)
		}
	})
	t.Run("anotacao na unica linha de dados", func(t *testing.T) {
		grade := [][]Celula{
			{txt("Nome"), txt("Telefone"), txt("Cidade")},
			{txt("João"), txt("9999-1234"), txt("Curitiba"), txt("cliente antigo")},
		}
		cols, _, motivo := analisarAba(grade)
		if motivo != "" {
			t.Fatal(motivo)
		}
		esperaColunas(t, cols, col("Nome", "TEXT"), col("Telefone", "TEXT"), col("Cidade", "TEXT"))
	})
}

// Título de duas células, linha em branco e cabeçalho de 3 colunas: a trava de "no máximo 2 células
// em cima" ainda deixa descer.
func TestTituloDeDuasCelulasComLinhaEmBrancoSobreCabecalho(t *testing.T) {
	grade := [][]Celula{
		{txt("Controle de Vendas"), num(2026)},
		{},
		{txt("Data"), txt("Produto"), txt("Qtd")},
		{data(2026, 9, 1), txt("Arroz"), num(2)},
	}
	cols, linhas, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	esperaColunas(t, cols, col("Data", "DATE"), col("Produto", "TEXT"), col("Qtd", "INTEGER"))
	if len(linhas) != 1 {
		t.Fatalf("linhas: %v", linhas)
	}
}

// Metadados de 3 ou mais células acima de um cabeçalho mais largo: a descida continua valendo, porque a
// linha de baixo é toda de títulos e o dado ocupa colunas além da extensão da de cima.
func TestMetadadosDeVariasCelulasSobreCabecalhoMaisLargo(t *testing.T) {
	t.Run("tres celulas", func(t *testing.T) {
		grade := [][]Celula{
			{txt("Loja Centro"), txt("Setembro"), num(2026)},
			{txt("Data"), txt("Produto"), txt("Qtd"), txt("Valor"), txt("Pago")},
			{data(2026, 9, 1), txt("Arroz"), num(2), num(10.5), txt("sim")},
			{data(2026, 9, 2), txt("Feijão"), num(1), num(8), txt("não")},
		}
		cols, linhas, motivo := analisarAba(grade)
		if motivo != "" {
			t.Fatal(motivo)
		}
		esperaColunas(t, cols, col("Data", "DATE"), col("Produto", "TEXT"), col("Qtd", "INTEGER"), col("Valor", "NUMERIC"), col("Pago", "TEXT"))
		want := [][]any{{"2026-09-01", "Arroz", int64(2), 10.5, "sim"}, {"2026-09-02", "Feijão", int64(1), 8.0, "não"}}
		if !reflect.DeepEqual(linhas, want) {
			t.Fatalf("linhas: %v", linhas)
		}
	})
	t.Run("quatro celulas e linha em branco", func(t *testing.T) {
		grade := [][]Celula{
			{txt("Loja:"), txt("Centro"), txt("Mês:"), txt("Setembro")},
			{},
			{txt("Data"), txt("Produto"), txt("Qtd"), txt("Valor"), txt("Pago"), txt("Vendedor")},
			{data(2026, 9, 1), txt("Arroz"), num(2), num(10.5), txt("sim"), txt("Ana")},
		}
		cols, linhas, motivo := analisarAba(grade)
		if motivo != "" {
			t.Fatal(motivo)
		}
		esperaColunas(t, cols, col("Data", "DATE"), col("Produto", "TEXT"), col("Qtd", "INTEGER"), col("Valor", "NUMERIC"), col("Pago", "TEXT"), col("Vendedor", "TEXT"))
		if !reflect.DeepEqual(linhas, [][]any{{"2026-09-01", "Arroz", int64(2), 10.5, "sim", "Ana"}}) {
			t.Fatalf("linhas: %v", linhas)
		}
	})
}

// Cabeçalho de 2 colunas com anotação na cauda da primeira linha de dados: a linha de baixo tem número,
// não é só título, então não vira cabeçalho.
func TestCabecalhoDeDuasColunasComAnotacaoNaCauda(t *testing.T) {
	grade := [][]Celula{
		{txt("Produto"), txt("Preço")},
		{txt("Arroz"), num(10), txt("promo")},
		{txt("Feijão"), num(8)},
	}
	cols, linhas, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	esperaColunas(t, cols, col("Produto", "TEXT"), col("Preco", "INTEGER"))
	if len(linhas) != 2 {
		t.Fatalf("linhas: %v", linhas)
	}
}

// O recuo para a esquerda só anda por células contíguas: "obs" em A, longe do cabeçalho em D..F, não cria
// Coluna_B e Coluna_C vazias.
func TestRecuoDaColunaSoPorCelulasContiguas(t *testing.T) {
	grade := [][]Celula{
		{vazia(), vazia(), vazia(), txt("Nome"), txt("Qtd"), txt("Obs")},
		{txt("obs"), vazia(), vazia(), txt("Arroz"), num(2), txt("x")},
		{vazia(), vazia(), vazia(), txt("Feijão"), num(1), txt("y")},
	}
	cols, linhas, motivo := analisarAba(grade)
	if motivo != "" {
		t.Fatal(motivo)
	}
	esperaColunas(t, cols, col("Nome", "TEXT"), col("Qtd", "INTEGER"), col("Obs", "TEXT"))
	if len(linhas) != 2 {
		t.Fatalf("linhas: %v", linhas)
	}
}
