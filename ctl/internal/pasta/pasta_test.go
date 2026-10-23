package pasta

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLimpar(t *testing.T) {
	casos := map[string]string{
		"/Users/a/Loja/\n":    "/Users/a/Loja",
		"/Users/a/Loja\n":     "/Users/a/Loja",
		"C:\\dados\\loja\r\n": "C:\\dados\\loja",
		"C:\\dados\\loja\\":   "C:\\dados\\loja",
		"C:\\":                "C:\\",
		"/\n":                 "/",
		"/Volumes/Pen Drive/": "/Volumes/Pen Drive",
		"":                    "",
	}
	for in, want := range casos {
		if got := limpar(in); got != want {
			t.Errorf("limpar(%q) = %q, quero %q", in, got, want)
		}
	}
}

func TestInterpretar(t *testing.T) {
	cancela1 := porCodigo(1)
	if p, ok, err := interpretar("/x/Loja/\n", "", nil, cancela1); p != "/x/Loja" || !ok || err != nil {
		t.Fatalf("escolha: %q %v %v", p, ok, err)
	}
	if _, ok, err := interpretar("\n", "", nil, cancela1); ok || err != nil {
		t.Fatalf("saída vazia deve contar como cancelado: %v %v", ok, err)
	}
	ee := exec.Command("sh", "-c", "exit 1").Run()
	if _, ok, err := interpretar("", "", ee, cancela1); ok || err != nil {
		t.Fatalf("código de cancelar: %v %v", ok, err)
	}
	if _, ok, err := interpretar("", "execution error: User canceled. (-128)", ee, func(_ int, s string) bool { return strings.Contains(s, "-128") }); ok || err != nil {
		t.Fatalf("cancelar do osascript: %v %v", ok, err)
	}
	ee2 := exec.Command("sh", "-c", "exit 2").Run()
	if _, _, err := interpretar("", "  Gtk-WARNING: cannot open display\n", ee2, cancela1); err == nil || !strings.Contains(err.Error(), "cannot open display") {
		t.Fatalf("falha deve trazer o stderr: %v", err)
	}
	if _, _, err := interpretar("", "", errors.New("exec: not found"), cancela1); err == nil {
		t.Fatal("erro sem código de saída deve virar erro")
	}
}

func TestPastaInicial(t *testing.T) {
	dir := t.TempDir()
	if got := pastaInicial(dir); got != dir {
		t.Errorf("pasta que existe: %q", got)
	}
	if got := pastaInicial(`"` + dir + `"`); got != dir {
		t.Errorf("com aspas: %q", got)
	}
	home, _ := os.UserHomeDir()
	if got := pastaInicial(dir + "/nao-existe"); got != home {
		t.Errorf("pasta que não existe deve virar a pessoal: %q", got)
	}
	f := dir + "/arquivo.dbf"
	_ = os.WriteFile(f, nil, 0o600)
	if got := pastaInicial(f); got != home {
		t.Errorf("arquivo não é pasta: %q", got)
	}
	if got := pastaInicial(""); got != home {
		t.Errorf("vazio: %q", got)
	}
}
