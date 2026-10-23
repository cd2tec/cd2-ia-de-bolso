// Package dbf lê arquivos .dbf de uma pasta e grava uma cópia SQLite para o toolbox consultar.
// O arquivo original não é alterado: a abertura é só leitura e a linha apagada do DBF fica de fora.
package dbf

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Valentin-Kaiser/go-dbase/dbase"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/copia"
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
	usados := map[string]int{}
	var tabelas []copia.Tabela
	for _, nome := range nomes {
		tabela, err := ler(filepath.Join(pasta, nome))
		if err != nil {
			return err
		}
		tabela.Nome = copia.Unico(copia.Identificador(strings.TrimSuffix(nome, filepath.Ext(nome))), usados)
		tabelas = append(tabelas, tabela)
	}
	return copia.Gravar(destino, tabelas)
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

func ler(caminho string) (copia.Tabela, error) {
	tab, err := dbase.OpenTable(&dbase.Config{
		Filename:          caminho,
		TrimSpaces:        true,
		ReadOnly:          true,
		Untested:          true,
		InterpretCodePage: true,
	})
	if err != nil {
		return copia.Tabela{}, fmt.Errorf("Não consegui ler %s.", filepath.Base(caminho))
	}
	defer tab.Close()
	var cols []copia.Coluna
	usados := map[string]int{}
	for _, c := range tab.Columns() {
		cols = append(cols, copia.Coluna{
			Nome: copia.Unico(copia.Identificador(strings.TrimSpace(c.Name())), usados),
			Tipo: tipoSQLite(c),
		})
	}
	if len(cols) == 0 {
		return copia.Tabela{}, fmt.Errorf("O arquivo %s não tem colunas.", filepath.Base(caminho))
	}
	var linhas [][]any
	for !tab.EOF() {
		row, err := tab.Next()
		if err != nil {
			return copia.Tabela{}, fmt.Errorf("Não consegui ler %s.", filepath.Base(caminho))
		}
		if row.Deleted {
			continue
		}
		linha := make([]any, len(cols))
		for i, c := range tab.Columns() {
			v, err := row.ValueByName(strings.TrimSpace(c.Name()))
			if err != nil {
				return copia.Tabela{}, fmt.Errorf("Não consegui ler %s.", filepath.Base(caminho))
			}
			linha[i] = valorSQL(v)
		}
		linhas = append(linhas, linha)
	}
	return copia.Tabela{Colunas: cols, Linhas: linhas}, nil
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
