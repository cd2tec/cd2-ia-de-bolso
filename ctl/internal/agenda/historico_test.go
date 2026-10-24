package agenda

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const chaveT = "0123456789abcdef0123456789abcdef"

var t0 = time.Date(2026, 10, 24, 8, 0, 0, 0, time.UTC)

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
	h := NovoHistorico(t.TempDir())
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
	// Blocos de 64 KB: linhas grandes forçam várias leituras.
	g := NovoHistorico(t.TempDir())
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

func TestLinhaCortadaEPulada(t *testing.T) {
	dir := t.TempDir()
	h := NovoHistorico(dir)
	for i := 0; i < 3; i++ {
		h.Anexar(chaveT, "c1", exec("agendada"))
	}
	f, err := os.OpenFile(filepath.Join(dir, "agenda", chaveT, "c1.jsonl"), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"seq":4,"ti`)
	f.Close()
	h.Anexar(chaveT, "c1", exec("agendada"))
	is, mais, err := h.Pagina(chaveT, "c1", 0, 50)
	if err != nil || mais || len(is) != 4 {
		t.Fatalf("Pagina: %v mais=%v err=%v", seqs(is), mais, err)
	}
	if n, _ := h.Contar(chaveT, "c1"); n != 4 {
		t.Errorf("Contar = %d", n)
	}
}

func TestNovoEVisto(t *testing.T) {
	h := NovoHistorico(t.TempDir())
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
	h := NovoHistorico(t.TempDir())
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
	h := NovoHistorico(t.TempDir())
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
	dir := t.TempDir()
	h := NovoHistorico(dir)
	h.Anexar(chaveT, "c1", exec("agendada"))
	if err := h.Apagar(chaveT, "c1"); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{".jsonl", ".estado.json"} {
		if _, err := os.Stat(filepath.Join(dir, "agenda", chaveT, "c1"+s)); !os.IsNotExist(err) {
			t.Errorf("%s ficou: %v", s, err)
		}
	}
	if n, err := h.Contar(chaveT, "c1"); err != nil || n != 0 {
		t.Errorf("Contar = %d, %v", n, err)
	}
	if err := h.Apagar(chaveT, "c1"); err != nil {
		t.Errorf("Apagar de novo: %v", err)
	}
	e, _ := h.Anexar(chaveT, "c1", exec("agendada"))
	if e.Seq != 1 {
		t.Errorf("seq recomeça em %d", e.Seq)
	}
}

func TestAberta(t *testing.T) {
	dir := t.TempDir()
	h := NovoHistorico(dir)
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
	if runtime.GOOS != "windows" {
		for _, p := range []string{filepath.Join(dir, "agenda", "aberta.json"), filepath.Join(dir, "agenda", chaveT, "aberta.json")} {
			if fi, err := os.Stat(p); err != nil || fi.Mode().Perm() != 0o600 {
				t.Errorf("%s: %v %v", p, fi, err)
			}
		}
		if fi, _ := os.Stat(filepath.Join(dir, "agenda", chaveT)); fi.Mode().Perm() != 0o700 {
			t.Errorf("pasta %v", fi.Mode().Perm())
		}
	}
}

func TestNaoVistasLeDeTrasParaFrente(t *testing.T) {
	dir := t.TempDir()
	h := NovoHistorico(dir)
	for i := 0; i < 3; i++ {
		h.Anexar(chaveT, "c1", exec("agendada"))
	}
	h.MarcarVisto(chaveT, "c1")
	h.Anexar(chaveT, "c1", exec("manual"))
	h.Anexar(chaveT, "c1", exec("agendada"))
	// Uma linha "nova" lá no começo só seria contada por quem lesse o arquivo inteiro: a leitura
	// de trás para a frente para no primeiro seq já visto.
	arq := filepath.Join(dir, "agenda", chaveT, "c1.jsonl")
	raw, _ := os.ReadFile(arq)
	os.WriteFile(arq, append([]byte(`{"seq":99,"tipo":"agendada"}`+"\n"), raw...), 0o600)
	if n, err := h.NaoVistas(chaveT, "c1"); err != nil || n != 1 {
		t.Fatalf("NaoVistas = %d, %v; quer 1", n, err)
	}
}

func TestContarVemDoEstado(t *testing.T) {
	dir := t.TempDir()
	h := NovoHistorico(dir)
	for i := 0; i < 5; i++ {
		h.Anexar(chaveT, "c1", exec("agendada"))
	}
	arq := filepath.Join(dir, "agenda", chaveT, "c1.jsonl")
	arqEst := filepath.Join(dir, "agenda", chaveT, "c1.estado.json")
	// Contar não lê o .jsonl: com ele trocado, continua dizendo 5.
	os.WriteFile(arq, []byte("{}\n"), 0o600)
	if n, err := h.Contar(chaveT, "c1"); err != nil || n != 5 {
		t.Fatalf("Contar = %d, %v; quer 5", n, err)
	}
	// Estado antigo, sem o total: conta uma vez lendo o arquivo e grava.
	var linhas []byte
	for i := 1; i <= 3; i++ {
		linhas = append(linhas, []byte(`{"seq":`+itoa(i)+`,"tipo":"agendada"}`+"\n")...)
	}
	os.WriteFile(arq, linhas, 0o600)
	os.WriteFile(arqEst, []byte(`{"seq":3,"visto":0,"rodadas":null}`), 0o600)
	if n, err := h.Contar(chaveT, "c1"); err != nil || n != 3 {
		t.Fatalf("Contar (estado antigo) = %d, %v; quer 3", n, err)
	}
	if raw, _ := os.ReadFile(arqEst); !strings.Contains(string(raw), `"total":3`) {
		t.Fatalf("total não gravado: %s", raw)
	}
	os.WriteFile(arqEst, []byte(`{"seq":3,"visto":0,"rodadas":null}`), 0o600)
	if e, _ := h.Anexar(chaveT, "c1", exec("agendada")); e.Seq != 4 {
		t.Fatalf("seq %d", e.Seq)
	}
	if n, _ := h.Contar(chaveT, "c1"); n != 4 {
		t.Fatalf("Contar depois de Anexar com estado antigo = %d; quer 4", n)
	}
	// Sem histórico nenhum: 0 e nada criado.
	if n, err := h.Contar(chaveT, "c2"); err != nil || n != 0 {
		t.Fatalf("Contar vazio = %d, %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "agenda", chaveT, "c2.estado.json")); !os.IsNotExist(err) {
		t.Fatalf("Contar criou estado: %v", err)
	}
}

func TestLinhasIguaisATodasNaoVaoDuasVezes(t *testing.T) {
	dir := t.TempDir()
	h := NovoHistorico(dir)
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
	raw, _ := os.ReadFile(filepath.Join(dir, "agenda", chaveT, "c1.jsonl"))
	l := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if strings.Contains(l[0], `"linhas"`) || !strings.Contains(l[1], `"linhas"`) {
		t.Fatalf("no disco:\n%s", raw)
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
