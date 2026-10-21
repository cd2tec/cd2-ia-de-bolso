package notas

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSchemaEfetivo(t *testing.T) {
	cases := []struct {
		b    Banco
		want string
	}{
		{Banco{Motor: "postgres"}, "public"},
		{Banco{Motor: "postgres", Schema: "Erp"}, "Erp"},
		{Banco{Motor: "sqlserver"}, "dbo"},
		{Banco{Motor: "oracle", Usuario: "loja"}, "LOJA"},
		{Banco{Motor: "mariadb", Base: "erp"}, "erp"},
		{Banco{Motor: "firebird", Schema: "x"}, ""},
		{Banco{Motor: "dbf"}, ""},
	}
	for _, c := range cases {
		if got := SchemaEfetivo(c.b); got != c.want {
			t.Errorf("SchemaEfetivo(%+v) = %q, quero %q", c.b, got, c.want)
		}
	}
}

func TestChaveIgnoraCaixaDoHostESchemaPadrao(t *testing.T) {
	a := Chave(Banco{Motor: "postgres", Host: "Servidor", Porta: 5432, Base: "erp"})
	b := Chave(Banco{Motor: "postgres", Host: "servidor", Porta: 5432, Base: "erp", Schema: "public"})
	if a != b {
		t.Errorf("chaves diferentes: %s e %s", a, b)
	}
	if len(a) != 32 {
		t.Errorf("len = %d, quero 32", len(a))
	}
	if Chave(Banco{Motor: "postgres", Host: "Servidor", Porta: 5433, Base: "erp"}) == a {
		t.Error("porta diferente deveria mudar a chave")
	}
	if Chave(Banco{Motor: "postgres", Host: "127.0.0.1", Porta: 5432, Base: "erp"}) ==
		Chave(Banco{Motor: "postgres", Host: "localhost", Porta: 5432, Base: "erp"}) {
		t.Error("127.0.0.1 e localhost deveriam ter chaves diferentes")
	}
}

func TestLerSemArquivoDevolveVazio(t *testing.T) {
	got, err := New(t.TempDir()).Ler(Banco{Motor: "postgres", Base: "erp"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != vazio {
		t.Errorf("got %s", got)
	}
}

func TestGravarELer(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	b := Banco{Motor: "postgres", Host: "h", Porta: 5432, Base: "erp"}
	dados := []byte(`{"versao":1,"tabelas":{"clientes":"cadastro"},"consultas":[{"nome":"x"}]}`)
	if err := s.Gravar(b, dados); err != nil {
		t.Fatal(err)
	}
	got, err := s.Ler(b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, dados) {
		t.Errorf("got %s", got)
	}
	arq := filepath.Join(dir, "notas", Chave(b)+".json")
	info, err := os.Stat(arq)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("modo = %v, quero 0600", info.Mode().Perm())
	}
}

func TestGravarRecusaInvalido(t *testing.T) {
	s := New(t.TempDir())
	b := Banco{Motor: "postgres", Base: "erp"}
	ok := []byte(`{"versao":1,"tabelas":{},"consultas":[{"a":1}]}`)
	if err := s.Gravar(b, ok); err != nil {
		t.Fatal(err)
	}
	grande := append([]byte(`{"versao":1,"tabelas":{},"consultas":[],"x":"`), bytes.Repeat([]byte("a"), MaxDados)...)
	grande = append(grande, '"', '}')
	ruins := [][]byte{
		[]byte("não é json"),
		[]byte(`[]`),
		[]byte(`{"versao":2}`),
		[]byte(`{"versao":1,"tabelas":[],"consultas":[]}`),
		[]byte(`{"versao":1,"tabelas":{},"consultas":{}}`),
		grande,
	}
	for _, r := range ruins {
		if err := s.Gravar(b, r); !errors.Is(err, ErrDados) {
			t.Errorf("Gravar(%.30q) err = %v, quero ErrDados", r, err)
		}
	}
	got, _ := s.Ler(b)
	if !bytes.Equal(got, ok) {
		t.Errorf("arquivo anterior mudou: %s", got)
	}
}

func TestGravarNaoTemSenha(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	b := Banco{Motor: "postgres", Base: "erp", Usuario: "usuario-secreto"}
	if err := s.Gravar(b, []byte(vazio)); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "notas", Chave(b)+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "usuario-secreto") {
		t.Error("usuário vazou para o arquivo")
	}
}
