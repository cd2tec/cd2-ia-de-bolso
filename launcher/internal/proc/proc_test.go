package proc

import (
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestHelperProcess faz o papel de um programa qualquer: o próprio binário de
// teste é re-executado com PROC_FALSO, sem depender de sleep do sistema.
func TestHelperProcess(t *testing.T) {
	modo := os.Getenv("PROC_FALSO")
	if modo == "" {
		return
	}
	switch modo {
	case "fala":
		os.Stdout.WriteString("saida-ok\n")
		os.Stderr.WriteString("erro-ok\n")
		os.Stdout.WriteString("amb=" + os.Getenv("PROC_EXTRA") + "\n")
		os.Exit(0)
	case "dorme":
		time.Sleep(time.Minute)
	case "teimoso":
		signal.Ignore(syscall.SIGTERM)
		os.Stdout.WriteString("pronto\n")
		time.Sleep(time.Minute)
	}
	os.Exit(0)
}

func iniciar(t *testing.T, modo string, env ...string) (*Processo, string, string) {
	t.Helper()
	dir := t.TempDir()
	saida, erros := filepath.Join(dir, "s.log"), filepath.Join(dir, "e.log")
	env = append([]string{"PROC_FALSO=" + modo}, env...)
	p, err := Iniciar(os.Args[0], []string{"-test.run=TestHelperProcess"}, env, saida, erros)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Parar() })
	return p, saida, erros
}

func fechou(p *Processo, prazo time.Duration) bool {
	select {
	case <-p.Morreu():
		return true
	case <-time.After(prazo):
		return false
	}
}

func TestIniciarSaidasSeparadasEAmbiente(t *testing.T) {
	p, saida, erros := iniciar(t, "fala", "PROC_EXTRA=xyz")
	if !fechou(p, 10*time.Second) {
		t.Fatal("o processo não terminou")
	}
	s, _ := os.ReadFile(saida)
	e, _ := os.ReadFile(erros)
	if !strings.Contains(string(s), "saida-ok") || !strings.Contains(string(s), "amb=xyz") {
		t.Fatalf("saída = %q", s)
	}
	if strings.Contains(string(s), "erro-ok") || !strings.Contains(string(e), "erro-ok") {
		t.Fatalf("stdout = %q, stderr = %q", s, e)
	}
}

func TestIniciarErrosNoMesmoArquivo(t *testing.T) {
	dir := t.TempDir()
	saida := filepath.Join(dir, "s.log")
	for _, erros := range []string{"", saida} {
		if err := os.WriteFile(saida, []byte("lixo antigo"), 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := Iniciar(os.Args[0], []string{"-test.run=TestHelperProcess"}, []string{"PROC_FALSO=fala"}, saida, erros)
		if err != nil {
			t.Fatal(err)
		}
		if !fechou(p, 10*time.Second) {
			t.Fatal("o processo não terminou")
		}
		s, _ := os.ReadFile(saida)
		if strings.Contains(string(s), "lixo antigo") {
			t.Fatalf("o log não foi zerado: %q", s)
		}
		if !strings.Contains(string(s), "saida-ok") || !strings.Contains(string(s), "erro-ok") {
			t.Fatalf("stdout e stderr deviam estar no mesmo arquivo: %q", s)
		}
	}
}

func TestIniciarExecutavelInexistente(t *testing.T) {
	dir := t.TempDir()
	if _, err := Iniciar(filepath.Join(dir, "nao-existe"), nil, nil, filepath.Join(dir, "s.log"), ""); err == nil {
		t.Fatal("esperava erro")
	}
}

func TestPararIdempotente(t *testing.T) {
	p, _, _ := iniciar(t, "dorme")
	if err := p.Parar(); err != nil {
		t.Fatal(err)
	}
	if err := p.Parar(); err != nil {
		t.Fatalf("segundo Parar: %v", err)
	}
	if !fechou(p, time.Second) {
		t.Fatal("Morreu() devia estar fechado")
	}
}

func TestPararDepoisDeSairSozinho(t *testing.T) {
	p, _, _ := iniciar(t, "fala")
	if !fechou(p, 10*time.Second) {
		t.Fatal("o processo não terminou")
	}
	if err := p.Parar(); err != nil {
		t.Fatal(err)
	}
}

func TestPararMataQuemIgnoraTerm(t *testing.T) {
	antes := EsperaTerm
	EsperaTerm = 300 * time.Millisecond
	defer func() { EsperaTerm = antes }()
	p, saida, _ := iniciar(t, "teimoso")
	// espera o helper instalar o signal.Ignore antes do TERM
	limite := time.Now().Add(10 * time.Second)
	for time.Now().Before(limite) {
		if b, _ := os.ReadFile(saida); strings.Contains(string(b), "pronto") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := p.Parar(); err != nil {
		t.Fatal(err)
	}
	if !fechou(p, time.Second) {
		t.Fatal("o processo teimoso continuou vivo")
	}
}
