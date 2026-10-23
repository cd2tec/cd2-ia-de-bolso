// Package copia grava tabelas lidas de uma fonte (DBF, planilha) numa cópia SQLite para o toolbox consultar.
// Fica à parte das fontes para que todas gerem nomes de tabela e coluna do mesmo jeito: sem acento nem
// símbolo, porque é isso que o SQL escrito à mão e o modelo conseguem digitar sem aspas.
package copia

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
	_ "modernc.org/sqlite"
)

// Coluna é uma coluna da cópia; Tipo é o tipo de afinidade do SQLite (TEXT, NUMERIC, INTEGER, DATE).
type Coluna struct {
	Nome, Tipo string
}

// Tabela é o que a fonte entrega pronto para gravar: nomes já passados por Identificador e Unico.
type Tabela struct {
	Nome    string
	Colunas []Coluna
	Linhas  [][]any
}

// Gravar substitui o SQLite em destino por uma cópia com as tabelas dadas, tudo numa transação.
func Gravar(destino string, tabelas []Tabela) error {
	if err := os.MkdirAll(filepath.Dir(destino), 0o700); err != nil {
		return fmt.Errorf("Não consegui preparar a cópia.")
	}
	for _, sufixo := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(destino + sufixo)
	}
	db, err := sql.Open("sqlite", destino)
	if err != nil {
		return fmt.Errorf("Não consegui abrir a cópia.")
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("Não consegui abrir a cópia.")
	}
	for _, t := range tabelas {
		if err := gravarTabela(tx, t); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("Não consegui copiar %s.", t.Nome)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("Não consegui gravar a cópia.")
	}
	return nil
}

func gravarTabela(tx *sql.Tx, t Tabela) error {
	var defs []string
	var nomes []string
	for _, c := range t.Colunas {
		defs = append(defs, quote(c.Nome)+" "+c.Tipo)
		nomes = append(nomes, quote(c.Nome))
	}
	if _, err := tx.Exec("CREATE TABLE " + quote(t.Nome) + " (" + strings.Join(defs, ", ") + ")"); err != nil {
		return err
	}
	if len(t.Linhas) == 0 {
		return nil
	}
	marca := strings.TrimRight(strings.Repeat("?,", len(t.Colunas)), ",")
	ins, err := tx.Prepare("INSERT INTO " + quote(t.Nome) + " (" + strings.Join(nomes, ", ") + ") VALUES (" + marca + ")")
	if err != nil {
		return err
	}
	defer ins.Close()
	for _, linha := range t.Linhas {
		if _, err := ins.Exec(linha...); err != nil {
			return err
		}
	}
	return nil
}

// SemAcento decompõe o texto (NFD) e descarta as marcas de acento, deixando as letras-base.
func SemAcento(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Identificador transforma um nome qualquer num identificador SQL simples: tira o acento, troca
// espaço, hífen, ponto e barra por "_", descarta o resto e limita a 60 caracteres. Ganha o prefixo
// T_ quando começaria com dígito. O prefixo reservado sqlite_ fica por conta do Unico, que vê o nome final.
func Identificador(s string) string {
	var b strings.Builder
	for _, r := range SemAcento(s) {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		case unicode.IsSpace(r) || r == '-' || r == '.' || r == '/':
			b.WriteByte('_')
		}
	}
	out := b.String()
	for strings.Contains(out, "__") {
		out = strings.ReplaceAll(out, "__", "_")
	}
	out = strings.Trim(out, "_")
	if out == "" || out[0] >= '0' && out[0] <= '9' {
		out = "T_" + out
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

// Unico devolve nome sem repetir um já usado (o SQLite ignora a caixa), anotando-o em usados. O nome que
// começa com sqlite_ (em qualquer caixa) ganha T_ antes da checagem, porque o SQLite o reserva
// (CREATE TABLE sqlite_x falha); a regra mora aqui, e não no Identificador, porque o nome final da
// tabela (arquivo + "__" + aba) só existe depois dele.
func Unico(nome string, usados map[string]int) string {
	if strings.HasPrefix(strings.ToLower(nome), "sqlite_") {
		nome = "T_" + nome
	}
	base := nome
	for {
		chave := strings.ToUpper(base)
		if usados[chave] == 0 {
			usados[chave] = 1
			return base
		}
		usados[chave]++
		base = fmt.Sprintf("%s_%d", nome, usados[chave])
	}
}

func quote(nome string) string { return `"` + strings.ReplaceAll(nome, `"`, `""`) + `"` }
