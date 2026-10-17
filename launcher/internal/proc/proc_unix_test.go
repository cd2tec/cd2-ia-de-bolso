//go:build !windows

package proc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// O llamafile é um APE: começa com um cabeçalho de shell, sem "#!". O
// kernel do Mac recusa o exec (ENOEXEC) e o script de hoje só funciona
// porque o bash cai no sh nesse caso. O Iniciar tem de fazer o mesmo.
func TestIniciarExecutavelSemShebangPeloSh(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "ape falso")
	if err := os.WriteFile(exe, []byte("echo \"ape-ok $1 $2\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	saida := filepath.Join(dir, "s.log")
	p, err := Iniciar(exe, []string{"um", "dois tres"}, nil, saida, "")
	if err != nil {
		t.Fatalf("Iniciar: %v", err)
	}
	<-p.Morreu()
	b, _ := os.ReadFile(saida)
	if got := strings.TrimSpace(string(b)); got != "ape-ok um dois tres" {
		t.Errorf("saída = %q", got)
	}
}
