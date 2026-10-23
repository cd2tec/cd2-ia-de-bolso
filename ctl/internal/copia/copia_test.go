package copia

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestIdentificadorTranslitera(t *testing.T) {
	casos := map[string]string{
		"Descrição": "Descricao", "Preço (R$)": "Preco_R", "Nº": "N", "Valor Total": "Valor_Total",
		"data-venda.1": "data_venda_1", "01/09": "T_01_09", "2024": "T_2024", "": "T_", "a  b": "a_b",
		strings.Repeat("x", 70): strings.Repeat("x", 60),
	}
	for in, want := range casos {
		if got := Identificador(in); got != want {
			t.Errorf("%q: %q != %q", in, got, want)
		}
	}
}

func TestUnicoIgnoraCaixa(t *testing.T) {
	u := map[string]int{}
	if Unico("Vendas", u) != "Vendas" || Unico("vendas", u) != "vendas_2" || Unico("VENDAS", u) != "VENDAS_3" {
		t.Fatal("Unico não diferenciou nomes que só mudam na caixa")
	}
}

func TestGravarCriaTabelasETipos(t *testing.T) {
	destino := filepath.Join(t.TempDir(), "sub", "x.sqlite")
	tabelas := []Tabela{{
		Nome:    "Vendas",
		Colunas: []Coluna{{"Produto", "TEXT"}, {"Valor", "NUMERIC"}, {"Dia", "DATE"}},
		Linhas: [][]any{
			{"Arroz", 12.5, "2024-09-01"},
			{"Feijão", 8.0, nil},
		},
	}}
	if err := Gravar(destino, tabelas); err != nil {
		t.Fatalf("Gravar: %v", err)
	}
	db, err := sql.Open("sqlite", destino)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	tipos := map[string]string{}
	rows, err := db.Query(`SELECT name, type FROM pragma_table_info('Vendas')`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var n, ty string
		if err := rows.Scan(&n, &ty); err != nil {
			t.Fatal(err)
		}
		tipos[n] = ty
	}
	rows.Close()
	want := map[string]string{"Produto": "TEXT", "Valor": "NUMERIC", "Dia": "DATE"}
	for k, v := range want {
		if tipos[k] != v {
			t.Errorf("coluna %s: tipo %q, esperado %q", k, tipos[k], v)
		}
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "Vendas"`).Scan(&n); err != nil || n != 2 {
		t.Errorf("COUNT(*) = %d (%v), esperado 2", n, err)
	}
}

func TestGravarSubstituiDestinoAnterior(t *testing.T) {
	destino := filepath.Join(t.TempDir(), "x.sqlite")
	um := []Tabela{{Nome: "A", Colunas: []Coluna{{"c", "TEXT"}}}}
	outro := []Tabela{{Nome: "B", Colunas: []Coluna{{"c", "TEXT"}}}}
	if err := Gravar(destino, um); err != nil {
		t.Fatal(err)
	}
	if err := Gravar(destino, outro); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", destino)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'A'`).Scan(&n); err != nil || n != 0 {
		t.Errorf("a tabela antiga sobreviveu (n=%d, err=%v)", n, err)
	}
}
