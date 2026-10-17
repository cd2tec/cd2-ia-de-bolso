package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/manifesto"
)

const shaMotor = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func argumentos(t *testing.T, extra ...string) []string {
	t.Helper()
	zip := filepath.Join(t.TempDir(), "x.zip")
	if err := os.WriteFile(zip, []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	raiz := filepath.Join("..", "..", "..")
	return append([]string{
		"-prepare-unix", filepath.Join(raiz, "core", "prepare-unix.sh"),
		"-prepare-windows", filepath.Join(raiz, "core", "prepare-windows.ps1"),
		"-version-file", filepath.Join(raiz, "VERSION"),
		"-zip", zip,
		"-zip-url", "https://exemplo.test/x.zip",
		"-llamafile-url", "https://exemplo.test/llamafile",
		"-llamafile-bytes", "20",
		"-modelo-bytes", "30",
	}, extra...)
}

func TestManifestoLevaOSHADoMotorEPassaNoLer(t *testing.T) {
	var out bytes.Buffer
	if err := executar(argumentos(t, "-llamafile-sha256", strings.ToUpper(shaMotor)), &out); err != nil {
		t.Fatal(err)
	}
	m, err := manifesto.Ler(&out)
	if err != nil {
		t.Fatalf("o manifesto gerado não passa no Ler: %v", err)
	}
	if m.Llamafile.SHA256 != shaMotor || m.Llamafile.Bytes != 20 {
		t.Fatalf("llamafile = %+v", m.Llamafile)
	}
}

func TestManifestoSemSHADoMotorFalha(t *testing.T) {
	for _, sha := range []string{"", "abc", strings.Repeat("z", 64)} {
		args := argumentos(t)
		if sha != "" {
			args = append(args, "-llamafile-sha256", sha)
		}
		if err := executar(args, &bytes.Buffer{}); err == nil {
			t.Errorf("sha %q: esperava erro", sha)
		}
	}
}
