//go:build linux

package codigo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMachineIDValido(t *testing.T) {
	casos := map[string]bool{
		"0123456789abcdef0123456789abcdef\n": true,
		"0123456789ABCDEF0123456789ABCDEF":   true,
		"":                                   false,
		"uninitialized\n":                    false,
		"0123456789abcdef0123456789abcde":    false,
		"0123456789abcdef0123456789abcdeg":   false,
	}
	for s, quer := range casos {
		if _, ok := MachineIDValido(s); ok != quer {
			t.Errorf("MachineIDValido(%q) = %v, quer %v", s, ok, quer)
		}
	}
}

func TestLerUsaOPrimeiroValido(t *testing.T) {
	dir := t.TempDir()
	vazio, dbus := filepath.Join(dir, "machine-id"), filepath.Join(dir, "dbus-machine-id")
	os.WriteFile(vazio, []byte("uninitialized\n"), 0o644)
	os.WriteFile(dbus, []byte("0123456789abcdef0123456789abcdef\n"), 0o644)
	antes := arquivosMachineID
	arquivosMachineID = []string{filepath.Join(dir, "nao-existe"), vazio, dbus}
	defer func() { arquivosMachineID = antes }()

	cod, err := Ler()
	if err != nil {
		t.Fatal(err)
	}
	if cod != Derivar("0123456789abcdef0123456789abcdef") {
		t.Fatalf("código = %q", cod)
	}
}

func TestLerSemMachineIDFalha(t *testing.T) {
	antes := arquivosMachineID
	arquivosMachineID = []string{filepath.Join(t.TempDir(), "nao-existe")}
	defer func() { arquivosMachineID = antes }()
	if _, err := Ler(); err == nil {
		t.Fatal("devia falhar sem machine-id")
	}
}
