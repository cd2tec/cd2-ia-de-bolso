package dbconf

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode"
)

// Engines aceitos, na ordem da tela.
var Engines = []string{"postgres", "oracle", "mariadb", "sqlserver", "firebird", "dbf", "excel"}

var defaultPorts = map[string]int{"postgres": 5432, "oracle": 1521, "mariadb": 3306, "sqlserver": 1433, "firebird": 3050, "dbf": 0, "excel": 0}

const (
	msgControl = "não pode ter quebra de linha nem caractere de controle"
	msgPort    = "use um número de 1 a 65535"
	msgEngine  = "use postgres, oracle, mariadb, sqlserver, firebird, dbf ou excel"
)

// DefaultPort devolve a porta padrão do motor (0 se o motor não existe ou não usa porta, como o DBF).
func DefaultPort(engine string) int { return defaultPorts[engine] }

// EngineArquivo diz se o motor lê uma pasta de arquivos (DBF ou planilhas Excel) em vez de
// conectar a um servidor: a "base" é o caminho da pasta e não há host, porta, usuário nem senha.
func EngineArquivo(engine string) bool { return engine == "dbf" || engine == "excel" }

func knownEngine(engine string) bool {
	_, ok := defaultPorts[engine]
	return ok
}

// DefaultSchema segue as regras da v0.5.0 (core/db-env.sh): Postgres public, Oracle o usuário
// em maiúsculas, MariaDB o nome da base, SQL Server dbo. Firebird não tem schema.
func DefaultSchema(engine, user, name string) string {
	if EngineArquivo(engine) {
		return ""
	}
	switch engine {
	case "postgres":
		return "public"
	case "oracle":
		return strings.ToUpper(user)
	case "mariadb":
		return name
	case "sqlserver":
		return "dbo"
	}
	return ""
}

// Input é o corpo JSON de POST /config. Porta aceita número ou texto; Senha ausente ou vazia
// mantém a senha salva.
type Input struct {
	Motor   string          `json:"motor"`
	Host    string          `json:"host"`
	Porta   json.RawMessage `json:"porta"`
	Base    string          `json:"base"`
	Usuario string          `json:"usuario"`
	Senha   *string         `json:"senha"`
	Schema  string          `json:"schema"`
}

// FieldError é um campo inválido; Error() segue "<campo>: <motivo>".
type FieldError struct{ Field, Reason string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }

func bad(field, reason string) error { return &FieldError{Field: field, Reason: reason} }

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func digits(s string) bool {
	if s == "" || len(s) > 5 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// parsePort aceita 5432, "5432", "" e null. Devolve 0 quando é para usar o padrão.
func parsePort(raw json.RawMessage) (int, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return 0, nil
	}
	if strings.HasPrefix(s, `"`) {
		var txt string
		if err := json.Unmarshal(raw, &txt); err != nil {
			return 0, bad("porta", msgPort)
		}
		if hasControl(txt) {
			return 0, bad("porta", msgControl)
		}
		s = strings.TrimSpace(txt)
		if s == "" {
			return 0, nil
		}
	}
	if !digits(s) {
		return 0, bad("porta", msgPort)
	}
	n, _ := strconv.Atoi(s)
	if n < 1 || n > 65535 {
		return 0, bad("porta", msgPort)
	}
	return n, nil
}

// check confere uma Config já normalizada (porta como texto).
func check(c Config) error {
	if !knownEngine(c.Engine) {
		return bad("motor", msgEngine)
	}
	if EngineArquivo(c.Engine) {
		if hasControl(c.Name) {
			return bad("base", msgControl)
		}
		if c.Name == "" {
			return bad("base", "obrigatório")
		}
		return nil
	}
	fields := []struct{ name, v string }{
		{"host", c.Host}, {"porta", c.Port}, {"base", c.Name}, {"usuario", c.User}, {"senha", c.Password}, {"schema", c.Schema},
	}
	for _, f := range fields {
		if hasControl(f.v) {
			return bad(f.name, msgControl)
		}
	}
	if c.Host == "" {
		return bad("host", "obrigatório")
	}
	if len(c.Host) > 253 {
		return bad("host", "até 253 caracteres")
	}
	if strings.IndexFunc(c.Host, unicode.IsSpace) >= 0 {
		return bad("host", "não pode ter espaço")
	}
	if !digits(c.Port) {
		return bad("porta", msgPort)
	}
	if n, _ := strconv.Atoi(c.Port); n < 1 || n > 65535 {
		return bad("porta", msgPort)
	}
	if c.Name == "" {
		return bad("base", "obrigatório")
	}
	if c.User == "" {
		return bad("usuario", "obrigatório")
	}
	return nil
}

// Check confere uma Config lida do disco, com as mesmas regras da tela.
func (c Config) Check() error { return check(c) }

// Validate monta a configuração candidata a partir da tela. saved é a configuração em vigor
// (nil se não há nenhuma): senha ausente ou vazia herda a dela. Erro é sempre *FieldError.
func Validate(in Input, saved *Config) (Config, error) {
	raw := []struct{ name, v string }{
		{"motor", in.Motor}, {"host", in.Host}, {"base", in.Base}, {"usuario", in.Usuario}, {"schema", in.Schema},
	}
	for _, f := range raw {
		if hasControl(f.v) {
			return Config{}, bad(f.name, msgControl)
		}
	}
	if in.Senha != nil && hasControl(*in.Senha) {
		return Config{}, bad("senha", msgControl)
	}
	engine := strings.TrimSpace(in.Motor)
	if !knownEngine(engine) {
		return Config{}, bad("motor", msgEngine)
	}
	port, err := parsePort(in.Porta)
	if err != nil {
		return Config{}, err
	}
	if port == 0 {
		port = defaultPorts[engine]
	}
	c := Config{
		Engine: engine,
		Host:   strings.TrimSpace(in.Host),
		Port:   strconv.Itoa(port),
		Name:   strings.TrimSpace(in.Base),
		User:   strings.TrimSpace(in.Usuario),
		Schema: strings.TrimSpace(in.Schema),
	}
	if in.Senha != nil && *in.Senha != "" {
		c.Password = *in.Senha
	} else if saved != nil {
		c.Password = saved.Password
	}
	if c.Schema == "" {
		c.Schema = DefaultSchema(c.Engine, c.User, c.Name)
	}
	// O Oracle guarda o dono em maiúsculas; "consinco" não casa com owner = 'CONSINCO' nas receitas.
	if c.Engine == "oracle" {
		c.Schema = strings.ToUpper(c.Schema)
	}
	if err := check(c); err != nil {
		return Config{}, err
	}
	return c, nil
}
