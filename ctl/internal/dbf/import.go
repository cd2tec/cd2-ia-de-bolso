// Package dbf lê arquivos .dbf de uma pasta e grava uma cópia SQLite para o toolbox consultar.
// O arquivo original não é alterado: a abertura é só leitura e a linha apagada do DBF fica de fora.
package dbf

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/Valentin-Kaiser/go-dbase/dbase"
	_ "modernc.org/sqlite"
)

// Import lê os .dbf de pasta e regrava o SQLite em destino. Destino é substituído.
func Import(pasta, destino string) error {
	info, err := os.Stat(pasta)
	if err != nil {
		return fmt.Errorf("Não achei a pasta dos arquivos .dbf.")
	}
	if !info.IsDir() {
		return fmt.Errorf("A base do DBF é uma pasta com os arquivos .dbf, não um arquivo.")
	}
	nomes, err := arquivos(pasta)
	if err != nil {
		return err
	}
	if len(nomes) == 0 {
		return fmt.Errorf("Não achei arquivo .dbf nessa pasta.")
	}
	if err := os.MkdirAll(filepath.Dir(destino), 0o700); err != nil {
		return fmt.Errorf("Não consegui preparar a cópia do DBF.")
	}
	for _, sufixo := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(destino + sufixo)
	}
	db, err := sql.Open("sqlite", destino)
	if err != nil {
		return fmt.Errorf("Não consegui abrir a cópia do DBF.")
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("Não consegui abrir a cópia do DBF.")
	}
	usados := map[string]int{}
	for _, nome := range nomes {
		tabela, err := ler(filepath.Join(pasta, nome))
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		tabela.nome = unico(identificador(strings.TrimSuffix(nome, filepath.Ext(nome))), usados)
		if err := gravar(tx, tabela); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("Não consegui copiar %s.", nome)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("Não consegui gravar a cópia do DBF.")
	}
	return nil
}

type coluna struct {
	nome string
	tipo string
}

type tabela struct {
	nome    string
	colunas []coluna
	linhas  [][]any
}

func arquivos(pasta string) ([]string, error) {
	entradas, err := os.ReadDir(pasta)
	if err != nil {
		return nil, fmt.Errorf("Não consegui ler a pasta dos arquivos .dbf.")
	}
	var nomes []string
	for _, e := range entradas {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".dbf") {
			nomes = append(nomes, e.Name())
		}
	}
	sort.Strings(nomes)
	return nomes, nil
}

func ler(caminho string) (tabela, error) {
	tab, err := dbase.OpenTable(&dbase.Config{
		Filename:          caminho,
		TrimSpaces:        true,
		ReadOnly:          true,
		Untested:          true,
		InterpretCodePage: true,
	})
	if err != nil {
		return tabela{}, fmt.Errorf("Não consegui ler %s.", filepath.Base(caminho))
	}
	defer tab.Close()
	var cols []coluna
	usados := map[string]int{}
	for _, c := range tab.Columns() {
		cols = append(cols, coluna{
			nome: unico(identificador(strings.TrimSpace(c.Name())), usados),
			tipo: tipoSQLite(c),
		})
	}
	if len(cols) == 0 {
		return tabela{}, fmt.Errorf("O arquivo %s não tem colunas.", filepath.Base(caminho))
	}
	var linhas [][]any
	for !tab.EOF() {
		row, err := tab.Next()
		if err != nil {
			return tabela{}, fmt.Errorf("Não consegui ler %s.", filepath.Base(caminho))
		}
		if row.Deleted {
			continue
		}
		linha := make([]any, len(cols))
		for i, c := range tab.Columns() {
			v, err := row.ValueByName(strings.TrimSpace(c.Name()))
			if err != nil {
				return tabela{}, fmt.Errorf("Não consegui ler %s.", filepath.Base(caminho))
			}
			linha[i] = valorSQL(v)
		}
		linhas = append(linhas, linha)
	}
	return tabela{colunas: cols, linhas: linhas}, nil
}

func tipoSQLite(c *dbase.Column) string {
	switch dbase.DataType(c.DataType) {
	case dbase.Date, dbase.DateTime:
		return "DATE"
	case dbase.Float, dbase.Double, dbase.Currency:
		return "NUMERIC"
	case dbase.Numeric:
		if c.Decimals > 0 {
			return "NUMERIC"
		}
		return "INTEGER"
	case dbase.Integer:
		return "INTEGER"
	case dbase.Logical:
		return "INTEGER"
	default:
		return "TEXT"
	}
}

func valorSQL(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case time.Time:
		if t.IsZero() {
			return nil
		}
		if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
			return t.Format("2006-01-02")
		}
		return t.UTC().Format("2006-01-02 15:04:05")
	case bool:
		if t {
			return 1
		}
		return 0
	default:
		return v
	}
}

func gravar(tx *sql.Tx, t tabela) error {
	var defs []string
	var nomes []string
	for _, c := range t.colunas {
		defs = append(defs, quote(c.nome)+" "+c.tipo)
		nomes = append(nomes, quote(c.nome))
	}
	if _, err := tx.Exec("CREATE TABLE " + quote(t.nome) + " (" + strings.Join(defs, ", ") + ")"); err != nil {
		return err
	}
	if len(t.linhas) == 0 {
		return nil
	}
	marca := strings.TrimRight(strings.Repeat("?,", len(t.colunas)), ",")
	ins, err := tx.Prepare("INSERT INTO " + quote(t.nome) + " (" + strings.Join(nomes, ", ") + ") VALUES (" + marca + ")")
	if err != nil {
		return err
	}
	defer ins.Close()
	for _, linha := range t.linhas {
		if _, err := ins.Exec(linha...); err != nil {
			return err
		}
	}
	return nil
}

func identificador(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		case unicode.IsSpace(r) || r == '-' || r == '.':
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" || out[0] >= '0' && out[0] <= '9' {
		out = "T_" + out
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

func unico(nome string, usados map[string]int) string {
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
