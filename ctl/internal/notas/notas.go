// Package notas guarda, por banco, as notas da TI e as consultas salvas: um arquivo JSON por
// conexão na pasta de cache. O arquivo nunca leva senha nem usuário.
package notas

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// MaxDados é o maior arquivo aceito em Gravar.
const MaxDados = 1 << 20

// ErrDados é o erro de Gravar quando o conteúdo não tem o formato esperado.
var ErrDados = errors.New("notas: conteúdo inválido")

// vazio é o que Ler devolve quando ainda não há arquivo.
const vazio = `{"versao":1,"tabelas":{},"consultas":[]}`

// Banco identifica a conexão. O usuário só entra na conta do schema do Oracle; não vai para o JSON.
type Banco struct {
	Motor   string `json:"motor"`
	Host    string `json:"host"`
	Porta   int    `json:"porta"`
	Base    string `json:"base"`
	Usuario string `json:"-"`
	Schema  string `json:"schema"`
}

// SchemaEfetivo é o schema que a consulta enxerga de fato, com o padrão de cada motor.
func SchemaEfetivo(b Banco) string {
	switch b.Motor {
	case "postgres":
		if b.Schema != "" {
			return b.Schema
		}
		return "public"
	case "sqlserver":
		if b.Schema != "" {
			return b.Schema
		}
		return "dbo"
	case "oracle":
		if b.Schema != "" {
			return b.Schema
		}
		return strings.ToUpper(b.Usuario)
	case "mariadb", "mysql":
		return b.Base
	}
	return ""
}

// Chave é o nome do arquivo do banco: 32 hex do SHA-256 de motor, host, porta, base e schema.
func Chave(b Banco) string {
	s := fmt.Sprintf("%s|%s|%d|%s|%s", b.Motor, strings.ToLower(b.Host), b.Porta, b.Base, SchemaEfetivo(b))
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:16])
}

// Store lê e grava os arquivos dentro de <dir>/notas.
type Store struct {
	dir string
}

// New cria o Store; a pasta só nasce na primeira gravação.
func New(dir string) *Store {
	return &Store{dir: dir}
}

func (s *Store) caminho(b Banco) string {
	return filepath.Join(s.dir, "notas", Chave(b)+".json")
}

// Ler devolve o JSON gravado, ou o documento vazio se o banco ainda não tem notas.
func (s *Store) Ler(b Banco) (json.RawMessage, error) {
	raw, err := os.ReadFile(s.caminho(b))
	if errors.Is(err, fs.ErrNotExist) {
		return json.RawMessage(vazio), nil
	}
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

// Gravar valida o documento e troca o arquivo inteiro (tmp + rename).
func (s *Store) Gravar(b Banco, dados json.RawMessage) error {
	if err := validar(dados); err != nil {
		return err
	}
	arq := s.caminho(b)
	if err := os.MkdirAll(filepath.Dir(arq), 0o700); err != nil {
		return err
	}
	tmp := arq + ".tmp"
	if err := os.WriteFile(tmp, dados, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, arq); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// validar aceita só um objeto com versao 1, tabelas objeto e consultas array.
func validar(dados []byte) error {
	if len(dados) > MaxDados {
		return ErrDados
	}
	var doc struct {
		Versao    int             `json:"versao"`
		Tabelas   json.RawMessage `json:"tabelas"`
		Consultas json.RawMessage `json:"consultas"`
	}
	if err := json.Unmarshal(dados, &doc); err != nil || doc.Versao != 1 {
		return ErrDados
	}
	var tabelas map[string]json.RawMessage
	if json.Unmarshal(doc.Tabelas, &tabelas) != nil || tabelas == nil {
		return ErrDados
	}
	var consultas []json.RawMessage
	if json.Unmarshal(doc.Consultas, &consultas) != nil || consultas == nil {
		return ErrDados
	}
	return nil
}
