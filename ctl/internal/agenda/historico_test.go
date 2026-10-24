package agenda

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

const chaveT = "0123456789abcdef0123456789abcdef"

var t0 = time.Date(2026, 10, 24, 8, 0, 0, 0, time.UTC)

// novoHist abre o histórico em dir e fecha no fim do teste (no Windows a pasta temporária não
// sai com o banco aberto).
func novoHist(t *testing.T, dir string) *Historico {
	t.Helper()
	h := NovoHistorico(dir)
	t.Cleanup(func() { h.Fechar() })
	return h
}

// abrirDireto abre o historico.sqlite por fora, para conferir o que ficou gravado.
func abrirDireto(t *testing.T, dir string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, "historico.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// travarAnexar faz o Anexar da consulta id falhar (um gatilho no banco) até chamar destravar.
func travarAnexar(t *testing.T, h *Historico, id string) (destravar func()) {
	t.Helper()
	sqlH := func(q string) {
		t.Helper()
		h.mu.Lock()
		defer h.mu.Unlock()
		db, err := h.banco()
		if err == nil {
			_, err = db.Exec(q)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	sqlH(`CREATE TRIGGER trava BEFORE INSERT ON execucoes WHEN NEW.consulta = '` + id + `' BEGIN SELECT RAISE(ABORT, 'trava'); END`)
	return func() { sqlH(`DROP TRIGGER trava`) }
}

func exec(tipo string) Execucao {
	return Execucao{Tipo: tipo, Ref: t0, Em: t0, Valores: []string{"a"}, Resultado: &Resultado{Ok: true, Frase: "oi"}}
}

func seqs(is []Execucao) []int64 {
	var s []int64
	for _, e := range is {
		s = append(s, e.Seq)
	}
	return s
}

func TestAnexarEPaginar(t *testing.T) {
	h := novoHist(t, t.TempDir())
	for i := 1; i <= 120; i++ {
		e, err := h.Anexar(chaveT, "c1", exec("agendada"))
		if err != nil || e.Seq != int64(i) {
			t.Fatalf("Anexar %d: seq %d, err %v", i, e.Seq, err)
		}
	}
	confere := func(antes int64, primeiro, ultimo int64, n int, mais bool) {
		t.Helper()
		is, m, err := h.Pagina(chaveT, "c1", antes, 50)
		if err != nil || m != mais || len(is) != n || is[0].Seq != primeiro || is[len(is)-1].Seq != ultimo {
			t.Fatalf("Pagina(%d): %v mais=%v err=%v", antes, seqs(is), m, err)
		}
	}
	confere(0, 120, 71, 50, true)
	confere(71, 70, 21, 50, true)
	confere(21, 20, 1, 20, false)
	if n, _ := h.Contar(chaveT, "c1"); n != 120 {
		t.Errorf("Contar = %d", n)
	}
	// Linhas grandes.
	g := novoHist(t, t.TempDir())
	grande := exec("agendada")
	grande.Resultado.Frase = strings.Repeat("x", 30<<10)
	for i := 0; i < 10; i++ {
		g.Anexar(chaveT, "c1", grande)
	}
	is, mais, err := g.Pagina(chaveT, "c1", 0, 50)
	if err != nil || mais || len(is) != 10 || is[0].Seq != 10 || is[9].Seq != 1 {
		t.Fatalf("grandes: %v mais=%v err=%v", seqs(is), mais, err)
	}
}

func TestBancoCorrompidoRecriado(t *testing.T) {
	dir := t.TempDir()
	lixo := []byte(strings.Repeat("isto não é um banco\n", 300))
	if err := os.WriteFile(filepath.Join(dir, "historico.sqlite"), lixo, 0o600); err != nil {
		t.Fatal(err)
	}
	var avisos bytes.Buffer
	h := novoHist(t, dir)
	h.avisos = &avisos
	e, err := h.Anexar(chaveT, "c1", exec("agendada"))
	if err != nil || e.Seq != 1 {
		t.Fatalf("Anexar: seq %d, %v", e.Seq, err)
	}
	velhos, _ := filepath.Glob(filepath.Join(dir, "historico.sqlite.corrompido-*"))
	if len(velhos) != 1 {
		t.Fatalf("corrompido guardado: %v", velhos)
	}
	if raw, _ := os.ReadFile(velhos[0]); !bytes.Equal(raw, lixo) {
		t.Error("o arquivo guardado não é o corrompido")
	}
	if !strings.Contains(avisos.String(), "corrompido") {
		t.Errorf("aviso: %q", avisos.String())
	}
	if n, err := h.Contar(chaveT, "c1"); err != nil || n != 1 {
		t.Errorf("Contar = %d, %v", n, err)
	}
}

func TestPaginaCorrompidaRecriada(t *testing.T) {
	dir := t.TempDir()
	h := NovoHistorico(dir)
	grande := exec("agendada")
	grande.Resultado.Frase = strings.Repeat("x", 8<<10)
	for i := 0; i < 20; i++ {
		if _, err := h.Anexar(chaveT, "c1", grande); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.Fechar(); err != nil {
		t.Fatal(err)
	}
	// Estraga as páginas do meio (o cabeçalho fica bom): só o quick_check percebe.
	arq := filepath.Join(dir, "historico.sqlite")
	raw, err := os.ReadFile(arq)
	if err != nil || len(raw) < 64<<10 {
		t.Fatalf("banco pequeno demais: %d %v", len(raw), err)
	}
	for i := 4096; i < len(raw)-4096; i++ {
		raw[i] = 0xa5
	}
	os.WriteFile(arq, raw, 0o600)
	var avisos bytes.Buffer
	h2 := novoHist(t, dir)
	h2.avisos = &avisos
	if n, err := h2.Contar(chaveT, "c1"); err != nil || n != 0 {
		t.Fatalf("Contar depois de recriar = %d, %v", n, err)
	}
	if velhos, _ := filepath.Glob(filepath.Join(dir, "historico.sqlite.corrompido-*")); len(velhos) != 1 {
		t.Fatalf("corrompido guardado: %v (aviso %q)", velhos, avisos.String())
	}
}

func TestNovoEVisto(t *testing.T) {
	h := novoHist(t, t.TempDir())
	h.Anexar(chaveT, "c1", exec("agendada"))
	h.Anexar(chaveT, "c1", exec("manual"))
	h.Anexar(chaveT, "c1", exec("agendada"))
	if n, err := h.NaoVistas(chaveT, "c1"); err != nil || n != 2 {
		t.Fatalf("NaoVistas = %d, %v", n, err)
	}
	is, _, _ := h.Pagina(chaveT, "c1", 0, 50)
	if !is[0].Novo || is[1].Novo || !is[2].Novo {
		t.Errorf("Novo: %v %v %v", is[0].Novo, is[1].Novo, is[2].Novo)
	}
	if err := h.MarcarVisto(chaveT, "c1"); err != nil {
		t.Fatal(err)
	}
	if n, _ := h.NaoVistas(chaveT, "c1"); n != 0 {
		t.Errorf("NaoVistas = %d", n)
	}
	is, _, _ = h.Pagina(chaveT, "c1", 0, 50)
	for _, e := range is {
		if e.Novo {
			t.Errorf("seq %d ainda Novo", e.Seq)
		}
	}
	e, _ := h.Anexar(chaveT, "c1", exec("agendada"))
	if e.Seq != 4 {
		t.Errorf("seq depois do visto = %d", e.Seq)
	}
	if n, _ := h.NaoVistas(chaveT, "c1"); n != 1 {
		t.Errorf("NaoVistas = %d", n)
	}
}

func TestRodadaNaPerda(t *testing.T) {
	h := novoHist(t, t.TempDir())
	p := Execucao{Tipo: "perda", Ref: t0, Em: t0, Motivo: "fechada", N: 3}
	h.Anexar(chaveT, "c1", p)
	h.Anexar(chaveT, "c1", p)
	em := t0.Add(time.Hour)
	if err := h.MarcarRodada(chaveT, "c1", 1, em); err != nil {
		t.Fatal(err)
	}
	is, _, _ := h.Pagina(chaveT, "c1", 0, 50)
	if is[0].Rodada != nil || is[1].Rodada == nil || !is[1].Rodada.Equal(em) {
		t.Errorf("Rodada: %v %v", is[0].Rodada, is[1].Rodada)
	}
	if !is[0].Novo {
		t.Error("perda sem visto deve ser Novo")
	}
}

func TestIdInvalido(t *testing.T) {
	h := novoHist(t, t.TempDir())
	for _, id := range []string{"../x", "", "a/b", strings.Repeat("a", 65)} {
		if _, err := h.Anexar(chaveT, id, exec("agendada")); !errors.Is(err, ErrId) {
			t.Errorf("Anexar(%q) = %v", id, err)
		}
		if _, _, err := h.Pagina(chaveT, id, 0, 1); !errors.Is(err, ErrId) {
			t.Errorf("Pagina(%q) = %v", id, err)
		}
		if h.Apagar(chaveT, id) == nil || h.MarcarVisto(chaveT, id) == nil {
			t.Errorf("%q aceito", id)
		}
	}
	if _, err := h.Anexar("../..", "c1", exec("agendada")); !errors.Is(err, ErrId) {
		t.Errorf("chave ruim = %v", err)
	}
	if _, _, err := h.Aberta("ABC"); !errors.Is(err, ErrId) {
		t.Errorf("Aberta chave ruim = %v", err)
	}
}

func TestApagar(t *testing.T) {
	h := novoHist(t, t.TempDir())
	h.Anexar(chaveT, "c1", exec("agendada"))
	h.Anexar(chaveT, "c1", Execucao{Tipo: "perda", Ref: t0, Em: t0, N: 1})
	h.MarcarRodada(chaveT, "c1", 2, t0)
	h.Anexar(chaveT, "c2", exec("agendada"))
	if err := h.Apagar(chaveT, "c1"); err != nil {
		t.Fatal(err)
	}
	if n, err := h.Contar(chaveT, "c1"); err != nil || n != 0 {
		t.Errorf("Contar = %d, %v", n, err)
	}
	if is, mais, err := h.Pagina(chaveT, "c1", 0, 50); err != nil || mais || len(is) != 0 {
		t.Errorf("Pagina = %v %v %v", seqs(is), mais, err)
	}
	if n, err := h.NaoVistas(chaveT, "c1"); err != nil || n != 0 {
		t.Errorf("NaoVistas = %d, %v", n, err)
	}
	if n, _ := h.Contar(chaveT, "c2"); n != 1 {
		t.Errorf("Apagar levou a c2: %d", n)
	}
	if err := h.Apagar(chaveT, "c1"); err != nil {
		t.Errorf("Apagar de novo: %v", err)
	}
	e, _ := h.Anexar(chaveT, "c1", Execucao{Tipo: "perda", Ref: t0, Em: t0, N: 1})
	if e.Seq != 1 {
		t.Errorf("seq recomeça em %d", e.Seq)
	}
	h.Anexar(chaveT, "c1", Execucao{Tipo: "perda", Ref: t0, Em: t0, N: 1})
	if is, _, _ := h.Pagina(chaveT, "c1", 0, 50); len(is) != 2 || is[0].Rodada != nil {
		t.Errorf("rodada da perda apagada voltou: %v", is[0].Rodada)
	}
}

func TestAberta(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dados")
	h := novoHist(t, dir)
	b, q, err := h.Aberta(chaveT)
	if err != nil || !b.IsZero() || !q.IsZero() {
		t.Fatalf("nunca: %v %v %v", b, q, err)
	}
	if err := h.MarcarAberta(chaveT, t0); err != nil {
		t.Fatal(err)
	}
	outra := "fedcba9876543210fedcba9876543210"
	t1 := t0.Add(time.Hour)
	if err := h.MarcarAberta(outra, t1); err != nil {
		t.Fatal(err)
	}
	b, q, _ = h.Aberta(chaveT)
	if !b.Equal(t0) || !q.Equal(t1) {
		t.Errorf("banco %v qualquer %v", b, q)
	}
	t2 := t1.Add(time.Hour)
	if err := h.MarcarAbertaSemBanco(t2); err != nil {
		t.Fatal(err)
	}
	if b, q, _ = h.Aberta(chaveT); !b.Equal(t0) || !q.Equal(t2) {
		t.Errorf("sem banco: banco %v qualquer %v", b, q)
	}
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat(filepath.Join(dir, "historico.sqlite")); err != nil || fi.Mode().Perm() != 0o600 {
			t.Errorf("historico.sqlite: %v %v", fi, err)
		}
		if fi, err := os.Stat(dir); err != nil || fi.Mode().Perm() != 0o700 {
			t.Errorf("pasta: %v %v", fi, err)
		}
	}
}

func TestLinhasIguaisATodasNaoVaoDuasVezes(t *testing.T) {
	dir := t.TempDir()
	h := novoHist(t, dir)
	rows := []map[string]any{{"a": "1"}, {"a": "2"}}
	e := exec("agendada")
	e.Resultado = &Resultado{Ok: true, Colunas: []string{"a"}, Linhas: rows, Todas: rows}
	h.Anexar(chaveT, "c1", e)
	if e.Resultado.Linhas == nil {
		t.Fatal("Anexar mexeu no Resultado de quem chamou")
	}
	ordem := exec("agendada")
	ordem.Resultado = &Resultado{Ok: true, Colunas: []string{"a"}, Linhas: []map[string]any{{"a": "2"}, {"a": "1"}}, Todas: rows}
	h.Anexar(chaveT, "c1", ordem)
	var l [2]string
	for i := range l {
		if err := abrirDireto(t, dir).QueryRow(`SELECT json FROM execucoes WHERE chave = ? AND consulta = 'c1' AND seq = ?`, chaveT, i+1).Scan(&l[i]); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Contains(l[0], `"linhas"`) || !strings.Contains(l[1], `"linhas"`) {
		t.Fatalf("no disco:\n%s\n%s", l[0], l[1])
	}
	is, _, err := h.Pagina(chaveT, "c1", 0, 50)
	if err != nil || len(is) != 2 {
		t.Fatalf("Pagina: %v %v", seqs(is), err)
	}
	if got, _ := json.Marshal(is[1].Resultado.Linhas); string(got) != `[{"a":"1"},{"a":"2"}]` {
		t.Fatalf("linhas refeitas = %s", got)
	}
	if got, _ := json.Marshal(is[0].Resultado.Linhas); string(got) != `[{"a":"2"},{"a":"1"}]` {
		t.Fatalf("linhas próprias = %s", got)
	}
}

func TestEsquema(t *testing.T) {
	dir := t.TempDir()
	h := novoHist(t, dir)
	if n, err := h.Contar(chaveT, "c1"); err != nil || n != 0 {
		t.Fatalf("Contar vazio = %d, %v", n, err)
	}
	db := abrirDireto(t, dir)
	var v int
	var modo string
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil || v != versaoEsquema || v < 1 {
		t.Errorf("user_version = %d, %v", v, err)
	}
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&modo); err != nil || modo != "wal" {
		t.Errorf("journal_mode = %q, %v", modo, err)
	}
	for _, tab := range []string{"execucoes", "estado", "rodadas", "aberta"} {
		var nome string
		if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, tab).Scan(&nome); err != nil {
			t.Errorf("tabela %s: %v", tab, err)
		}
	}
	// Reabrir não recria nem perde nada.
	h.Anexar(chaveT, "c1", exec("agendada"))
	h.Fechar()
	if n, err := novoHist(t, dir).Contar(chaveT, "c1"); err != nil || n != 1 {
		t.Errorf("depois de reabrir: %d, %v", n, err)
	}
}

// Pen drive e instalada podem ter cada uma o seu cd2-ctl gravando no mesmo arquivo.
func TestDoisHistoricosNoMesmoArquivo(t *testing.T) {
	dir := t.TempDir()
	hs := []*Historico{novoHist(t, dir), novoHist(t, dir)}
	const porVez = 40
	var wg sync.WaitGroup
	erros := make(chan error, 4*porVez)
	for i, h := range hs {
		for _, id := range []string{"c1", fmt.Sprintf("so%d", i)} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < porVez; j++ {
					if _, err := h.Anexar(chaveT, id, exec("agendada")); err != nil {
						erros <- err
					}
					if _, err := h.NaoVistas(chaveT, id); err != nil {
						erros <- err
					}
					if err := h.MarcarAberta(chaveT, t0); err != nil {
						erros <- err
					}
				}
			}()
		}
	}
	wg.Wait()
	close(erros)
	for err := range erros {
		t.Fatalf("gravação em paralelo: %v", err)
	}
	if n, _ := hs[0].Contar(chaveT, "c1"); n != 2*porVez {
		t.Fatalf("Contar c1 = %d", n)
	}
	visto := map[int64]bool{}
	for antes := int64(0); ; {
		is, mais, err := hs[1].Pagina(chaveT, "c1", antes, 50)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range is {
			if visto[e.Seq] {
				t.Fatalf("seq %d repetido", e.Seq)
			}
			visto[e.Seq] = true
			antes = e.Seq
		}
		if !mais {
			break
		}
	}
	if len(visto) != 2*porVez || !visto[1] || !visto[2*porVez] {
		t.Fatalf("seqs: %d", len(visto))
	}
}

func TestFecharEReabrir(t *testing.T) {
	h := novoHist(t, t.TempDir())
	h.Anexar(chaveT, "c1", exec("agendada"))
	if err := h.Fechar(); err != nil {
		t.Fatal(err)
	}
	if err := h.Fechar(); err != nil {
		t.Errorf("Fechar de novo: %v", err)
	}
	if e, err := h.Anexar(chaveT, "c1", exec("agendada")); err != nil || e.Seq != 2 {
		t.Errorf("depois de Fechar: seq %d, %v", e.Seq, err)
	}
}
