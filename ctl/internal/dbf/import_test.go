package dbf

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func TestImportCP850IgnoraApagado(t *testing.T) {
	dir := t.TempDir()
	escreve(t, filepath.Join(dir, "CLIENTE.DBF"), 0x02, charmap.CodePage850, []reg{
		{"Padaria do Ze", "10.00", true},
		{"Some da lista", "1.00", false},
		{"Açougue São José", "240.90", true},
	})
	dest := filepath.Join(dir, "out", "dbf.sqlite")
	if err := Import(dir, dest); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dest)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var tipo string
	if err := db.QueryRow(`SELECT type FROM pragma_table_info('CLIENTE') WHERE name = 'VALOR'`).Scan(&tipo); err != nil {
		t.Fatal(err)
	}
	if tipo != "NUMERIC" {
		t.Fatalf("tipo do valor: %s", tipo)
	}
	var nome string
	var total float64
	var qtd int
	err = db.QueryRow(`SELECT COUNT(*), ROUND(SUM(VALOR), 2) FROM CLIENTE`).Scan(&qtd, &total)
	if err != nil {
		t.Fatal(err)
	}
	if qtd != 2 || total != 250.90 {
		t.Fatalf("qtd=%d total=%v", qtd, total)
	}
	err = db.QueryRow(`SELECT NOME FROM CLIENTE WHERE NOME LIKE '%José'`).Scan(&nome)
	if err != nil {
		t.Fatal(err)
	}
	if nome != "Açougue São José" {
		t.Fatalf("acento: %q", nome)
	}
	var col, tipoCol string
	err = db.QueryRow(`
		WITH p AS (SELECT 'valor' AS pattern)
		SELECT c.name, c.type
		  FROM sqlite_master m
		  JOIN pragma_table_info(m.name) c
		  JOIN p
		 WHERE m.name = 'CLIENTE' AND UPPER(c.name) LIKE '%' || UPPER(p.pattern) || '%'`).Scan(&col, &tipoCol)
	if err != nil {
		t.Fatal(err)
	}
	if col != "VALOR" || tipoCol != "NUMERIC" {
		t.Fatalf("coluna %s %s", col, tipoCol)
	}
}

func TestImportRecusaPastaVazia(t *testing.T) {
	if err := Import(t.TempDir(), filepath.Join(t.TempDir(), "x.sqlite")); err == nil {
		t.Fatal("pasta vazia passou")
	}
}

type reg struct {
	nome  string
	valor string
	ativo bool
}

func escreve(t *testing.T, path string, pagina byte, enc *charmap.Charmap, linhas []reg) {
	t.Helper()
	const nomeL, valorL = 24, 10
	var buf bytes.Buffer
	h := make([]byte, 32)
	h[0] = 0x03
	h[29] = pagina
	binary.LittleEndian.PutUint32(h[4:], uint32(len(linhas)))
	binary.LittleEndian.PutUint16(h[8:], 32+64+1)
	binary.LittleEndian.PutUint16(h[10:], 1+nomeL+valorL)
	buf.Write(h)
	buf.Write(campo("NOME", 'C', nomeL, 0))
	buf.Write(campo("VALOR", 'N', valorL, 2))
	buf.WriteByte(0x0D)
	coder := enc.NewEncoder()
	for _, l := range linhas {
		if l.ativo {
			buf.WriteByte(0x20)
		} else {
			buf.WriteByte(0x2A)
		}
		raw, err := coder.String(l.nome)
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(fixo(raw, nomeL))
		buf.Write(fixo(l.valor, valorL))
	}
	buf.WriteByte(0x1A)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func campo(nome string, tipo byte, tam, dec int) []byte {
	b := make([]byte, 32)
	copy(b, nome)
	b[11] = tipo
	b[16] = byte(tam)
	b[17] = byte(dec)
	return b
}

func fixo(s string, n int) []byte {
	b := bytes.Repeat([]byte{' '}, n)
	copy(b, s)
	return b
}
