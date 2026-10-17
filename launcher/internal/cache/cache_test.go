package cache

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPrepararCopiaSoQuandoMuda(t *testing.T) {
	dir := t.TempDir()
	origem := filepath.Join(dir, "origem")
	destino := filepath.Join(dir, "cache", "destino")
	if err := os.WriteFile(origem, []byte("motor v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	copiou, err := Preparar(origem, destino)
	if err != nil || !copiou {
		t.Fatalf("1a: copiou = %v, err = %v", copiou, err)
	}
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(destino)
		if st.Mode().Perm() != 0o755 {
			t.Fatalf("modo = %v, esperava 0755", st.Mode().Perm())
		}
	}
	copiou, err = Preparar(origem, destino)
	if err != nil || copiou {
		t.Fatalf("2a: copiou = %v, err = %v", copiou, err)
	}
	// Mesmo tamanho, 1 byte diferente: só a comparação do conteúdo pega.
	if err := os.WriteFile(origem, []byte("motor v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	copiou, err = Preparar(origem, destino)
	if err != nil || !copiou {
		t.Fatalf("3a: copiou = %v, err = %v", copiou, err)
	}
	b, _ := os.ReadFile(destino)
	if string(b) != "motor v2" {
		t.Fatalf("conteúdo = %q", b)
	}
	if _, err := os.Stat(destino + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("sobrou o arquivo .tmp")
	}
}

func TestPrepararOrigemInexistente(t *testing.T) {
	dir := t.TempDir()
	if _, err := Preparar(filepath.Join(dir, "nao-existe"), filepath.Join(dir, "d")); err == nil {
		t.Fatal("esperava erro")
	}
}

func TestPastaPorSistema(t *testing.T) {
	win := func(k string) string {
		return map[string]string{"LOCALAPPDATA": `C:\U\AppData\Local`, "TEMP": `C:\T`}[k]
	}
	if got := Pasta("windows", win, ""); got != `C:\U\AppData\Local\cd2-harness` {
		t.Errorf("Pasta windows = %q", got)
	}
	if got := Pasta("darwin", nil, "/Users/g"); got != "/Users/g/Library/Caches/cd2-harness" {
		t.Errorf("Pasta darwin = %q", got)
	}
	if got := PastaLogs("windows", win, ""); got != `C:\T\cd2-harness` {
		t.Errorf("PastaLogs windows = %q", got)
	}
	if got := PastaLogs("darwin", nil, "/Users/g"); got != "/Users/g/Library/Caches/cd2-harness" {
		t.Errorf("PastaLogs darwin = %q", got)
	}
}
