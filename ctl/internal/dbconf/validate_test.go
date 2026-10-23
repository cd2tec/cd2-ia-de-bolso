package dbconf

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func str(s string) *string { return &s }

func good() Input {
	return Input{Motor: "postgres", Host: "db.local", Porta: json.RawMessage(`5432`), Base: "cd2", Usuario: "leitor", Senha: str("x")}
}

func TestValidatePreencheOsPadroes(t *testing.T) {
	cases := []struct {
		in           Input
		port, schema string
	}{
		{Input{Motor: "postgres", Host: "h", Base: "cd2", Usuario: "u"}, "5432", "public"},
		{Input{Motor: "oracle", Host: "h", Base: "FREEPDB1", Usuario: "leitor"}, "1521", "LEITOR"},
		{Input{Motor: "mariadb", Host: "h", Base: "loja", Usuario: "u"}, "3306", "loja"},
		{Input{Motor: "sqlserver", Host: "h", Base: "d", Usuario: "u"}, "1433", "dbo"},
		{Input{Motor: "firebird", Host: "h", Base: `C:\dados\erp.fdb`, Usuario: "LEITURA_RO"}, "3050", ""},
		{Input{Motor: "dbf", Base: `C:\dados\loja`}, "0", ""},
		{Input{Motor: "postgres", Host: "h", Porta: json.RawMessage(`"6543"`), Base: "d", Usuario: "u", Schema: "vendas"}, "6543", "vendas"},
		{Input{Motor: "oracle", Host: "h", Base: "PRD", Usuario: "CONSINCO", Schema: "consinco"}, "1521", "CONSINCO"},
		{Input{Motor: "postgres", Host: "h", Porta: json.RawMessage(`""`), Base: "d", Usuario: "u"}, "5432", "public"},
		{Input{Motor: "postgres", Host: "h", Porta: json.RawMessage(`null`), Base: "d", Usuario: "u"}, "5432", "public"},
	}
	for _, tc := range cases {
		c, err := Validate(tc.in, nil)
		if err != nil {
			t.Errorf("%s: %v", tc.in.Motor, err)
			continue
		}
		if c.Port != tc.port || c.Schema != tc.schema {
			t.Errorf("%s: porta %q schema %q, quero %q %q", tc.in.Motor, c.Port, c.Schema, tc.port, tc.schema)
		}
	}
}

func TestValidateAparaMenosASenha(t *testing.T) {
	in := good()
	in.Host, in.Usuario, in.Base, in.Schema, in.Senha = "  db.local ", " leitor", "cd2 ", " public ", str(" com espaço ")
	c, err := Validate(in, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := Config{Engine: "postgres", Host: "db.local", Port: "5432", Name: "cd2", User: "leitor", Password: " com espaço ", Schema: "public"}
	if c != want {
		t.Errorf("Validate = %+v", c)
	}
}

func TestValidateRecusaComMensagemPorCampo(t *testing.T) {
	cases := []struct {
		mut  func(*Input)
		want string
	}{
		{func(i *Input) { i.Motor = "mysql" }, "motor: use postgres, oracle, mariadb, sqlserver, firebird, dbf ou excel"},
		{func(i *Input) { i.Host = "" }, "host: obrigatório"},
		{func(i *Input) { i.Host = strings.Repeat("a", 254) }, "host: até 253 caracteres"},
		{func(i *Input) { i.Host = "db local" }, "host: não pode ter espaço"},
		{func(i *Input) { i.Host = "db.local\n" }, "host: não pode ter quebra de linha nem caractere de controle"},
		{func(i *Input) { i.Porta = json.RawMessage(`0`) }, "porta: use um número de 1 a 65535"},
		{func(i *Input) { i.Porta = json.RawMessage(`65536`) }, "porta: use um número de 1 a 65535"},
		{func(i *Input) { i.Porta = json.RawMessage(`5432.5`) }, "porta: use um número de 1 a 65535"},
		{func(i *Input) { i.Porta = json.RawMessage(`-1`) }, "porta: use um número de 1 a 65535"},
		{func(i *Input) { i.Porta = json.RawMessage(`"abc"`) }, "porta: use um número de 1 a 65535"},
		{func(i *Input) { i.Porta = json.RawMessage(`"12 34"`) }, "porta: use um número de 1 a 65535"},
		{func(i *Input) { i.Base = "  " }, "base: obrigatório"},
		{func(i *Input) { i.Base = "cd2\x00" }, "base: não pode ter quebra de linha nem caractere de controle"},
		{func(i *Input) { i.Usuario = "" }, "usuario: obrigatório"},
		{func(i *Input) { i.Senha = str("a\nb") }, "senha: não pode ter quebra de linha nem caractere de controle"},
		{func(i *Input) { i.Schema = "pub\tlic" }, "schema: não pode ter quebra de linha nem caractere de controle"},
	}
	for _, tc := range cases {
		in := good()
		tc.mut(&in)
		_, err := Validate(in, nil)
		var fe *FieldError
		if !errors.As(err, &fe) {
			t.Errorf("quero %q, veio %v", tc.want, err)
			continue
		}
		if fe.Error() != tc.want {
			t.Errorf("veio %q, quero %q", fe.Error(), tc.want)
		}
	}
}

func TestValidateSenhaVaziaMantemASalva(t *testing.T) {
	saved := &Config{Password: "antiga"}
	for _, s := range []*string{nil, str("")} {
		in := good()
		in.Senha = s
		c, err := Validate(in, saved)
		if err != nil {
			t.Fatal(err)
		}
		if c.Password != "antiga" {
			t.Errorf("senha %v: ficou %q", s, c.Password)
		}
	}
	c, _ := Validate(good(), saved)
	if c.Password != "x" {
		t.Errorf("senha nova não valeu: %q", c.Password)
	}
	in := good()
	in.Senha = nil
	c, _ = Validate(in, nil)
	if c.Password != "" {
		t.Errorf("sem salva, vazia deveria ser vazia: %q", c.Password)
	}
}

func TestCheck(t *testing.T) {
	if err := sample("x").Check(); err != nil {
		t.Errorf("config boa recusada: %v", err)
	}
	if err := (Config{Engine: "postgres"}).Check(); err == nil || err.Error() != "host: obrigatório" {
		t.Errorf("host vazio: %v", err)
	}
	bad := sample("x")
	bad.Port = "abc"
	if err := bad.Check(); err == nil {
		t.Error("porta abc passou")
	}
	bad = sample("x")
	bad.Engine = "db2"
	if err := bad.Check(); err == nil {
		t.Error("motor desconhecido passou")
	}
}

func TestPadroes(t *testing.T) {
	if DefaultPort("oracle") != 1521 || DefaultPort("xpto") != 0 {
		t.Error("DefaultPort")
	}
	if DefaultSchema("oracle", "leitor", "") != "LEITOR" || DefaultSchema("mariadb", "u", "loja") != "loja" || DefaultSchema("sqlserver", "", "") != "dbo" || DefaultSchema("firebird", "SYSDBA", "x") != "" || DefaultSchema("dbf", "", `C:\dados`) != "" {
		t.Error("DefaultSchema")
	}
	if len(Engines) != 7 || Engines[5] != "dbf" || Engines[6] != "excel" {
		t.Error("Engines")
	}
}

func TestMotorExcel(t *testing.T) {
	c, err := Validate(Input{Motor: "excel", Base: `C:\planilhas`}, nil)
	if err != nil {
		t.Fatalf("excel só com a base foi recusado: %v", err)
	}
	if c.Engine != "excel" || c.Name != `C:\planilhas` || c.Port != "0" || c.Schema != "" {
		t.Errorf("config do excel: %+v", c)
	}
	_, err = Validate(Input{Motor: "excel"}, nil)
	if err == nil || err.Error() != "base: obrigatório" {
		t.Errorf("base vazia: %v", err)
	}
	if DefaultPort("excel") != 0 {
		t.Error("DefaultPort(excel) deveria ser 0")
	}
	if DefaultSchema("excel", "u", "x") != "" {
		t.Error("DefaultSchema(excel) deveria ser vazio")
	}
	if !EngineArquivo("excel") || !EngineArquivo("dbf") || EngineArquivo("postgres") || EngineArquivo("xpto") {
		t.Error("EngineArquivo")
	}
	_, err = Validate(Input{Motor: "db2"}, nil)
	if err == nil || !strings.Contains(err.Error(), "excel") {
		t.Errorf("msgEngine deveria citar excel: %v", err)
	}
}
