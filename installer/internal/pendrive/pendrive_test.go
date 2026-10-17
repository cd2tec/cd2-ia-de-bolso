package pendrive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func escrever(t *testing.T, raiz, rel, conteudo string) {
	t.Helper()
	c := filepath.Join(raiz, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(c), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

func soma(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// montaStage cria um stage pequeno com CHECKSUMS.sha256 coerente.
func montaStage(t *testing.T) string {
	t.Helper()
	st := t.TempDir()
	escrever(t, st, "bin/llamafile", "binario")
	escrever(t, st, "models/x.gguf", "modelo grande")
	escrever(t, st, "LEIA.txt", "oi")
	escrever(t, st, "._x", "lixo")
	escrever(t, st, "bin/.DS_Store", "lixo")
	cs := soma("binario") + "  bin/llamafile\n" + soma("modelo grande") + "  models/x.gguf\n"
	escrever(t, st, "CHECKSUMS.sha256", cs)
	return st
}

func TestCopiarArvore(t *testing.T) {
	st := montaStage(t)
	dst := t.TempDir()
	// lixo já existente no destino também deve sumir
	escrever(t, dst, "._velho", "x")
	escrever(t, dst, "models/.DS_Store", "x")

	total, err := TamanhoTotal(st)
	if err != nil {
		t.Fatal(err)
	}
	var ultimoFeitos, ultimoTotal int64
	if err := Copiar(context.Background(), st, dst, func(f, tt int64) { ultimoFeitos, ultimoTotal = f, tt }); err != nil {
		t.Fatal(err)
	}
	if ultimoTotal != total || ultimoFeitos != total {
		t.Fatalf("progresso %d/%d, esperava %d", ultimoFeitos, ultimoTotal, total)
	}
	for _, rel := range []string{"bin/llamafile", "models/x.gguf", "LEIA.txt", "CHECKSUMS.sha256"} {
		a, _ := os.ReadFile(filepath.Join(st, rel))
		b, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil || !bytes.Equal(a, b) {
			t.Fatalf("%s diferente ou ausente: %v", rel, err)
		}
	}
	for _, rel := range []string{"._x", "bin/.DS_Store", "._velho", "models/.DS_Store"} {
		if _, err := os.Stat(filepath.Join(dst, rel)); err == nil {
			t.Fatalf("%s não deveria existir no destino", rel)
		}
	}
}

func TestCopiarDestinoSumiu(t *testing.T) {
	st := montaStage(t)
	dst := filepath.Join(t.TempDir(), "pen")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	removeu := false
	err := Copiar(context.Background(), st, dst, func(f, tt int64) {
		if !removeu {
			removeu = true
			os.RemoveAll(dst)
		}
	})
	if !errors.Is(err, ErrRemovido) {
		t.Fatalf("esperava ErrRemovido, veio %v", err)
	}
}

func TestCopiarCancelado(t *testing.T) {
	st := montaStage(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Copiar(ctx, st, t.TempDir(), nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("esperava Canceled, veio %v", err)
	}
}

func TestConferirOk(t *testing.T) {
	st := montaStage(t)
	dst := t.TempDir()
	if err := Copiar(context.Background(), st, dst, nil); err != nil {
		t.Fatal(err)
	}
	var feitos int64
	if err := Conferir(context.Background(), dst, func(f, _ int64) { feitos = f }); err != nil {
		t.Fatal(err)
	}
	if want := int64(len("binario") + len("modelo grande")); feitos != want {
		t.Fatalf("feitos %d, esperava %d", feitos, want)
	}
}

func TestConferirDiferente(t *testing.T) {
	st := montaStage(t)
	dst := t.TempDir()
	if err := Copiar(context.Background(), st, dst, nil); err != nil {
		t.Fatal(err)
	}
	escrever(t, dst, "models/x.gguf", "modelo grandX")
	err := Conferir(context.Background(), dst, nil)
	if !errors.Is(err, ErrDiferente) || !strings.Contains(err.Error(), "models/x.gguf") {
		t.Fatalf("esperava ErrDiferente com caminho, veio %v", err)
	}
}

func TestConferirArquivoFaltando(t *testing.T) {
	st := montaStage(t)
	dst := t.TempDir()
	if err := Copiar(context.Background(), st, dst, nil); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(dst, "bin", "llamafile"))
	err := Conferir(context.Background(), dst, nil)
	if !errors.Is(err, ErrDiferente) || !strings.Contains(err.Error(), "bin/llamafile") {
		t.Fatalf("esperava ErrDiferente com caminho, veio %v", err)
	}
}

func TestConferirDestinoSumiu(t *testing.T) {
	st := montaStage(t)
	dst := filepath.Join(t.TempDir(), "pen")
	os.MkdirAll(dst, 0o755)
	if err := Copiar(context.Background(), st, dst, nil); err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(dst)
	if err := Conferir(context.Background(), dst, nil); !errors.Is(err, ErrRemovido) {
		t.Fatalf("esperava ErrRemovido, veio %v", err)
	}
}

func TestConferirSemChecksums(t *testing.T) {
	if err := Conferir(context.Background(), t.TempDir(), nil); !errors.Is(err, ErrDiferente) {
		t.Fatalf("esperava ErrDiferente, veio %v", err)
	}
}

func TestGuardarChecksumsNome(t *testing.T) {
	dst := t.TempDir()
	escrever(t, dst, "CHECKSUMS.sha256", "abc  x\n")
	cache := filepath.Join(t.TempDir(), "nova", "cache")
	agora := time.Date(2026, 10, 16, 14, 30, 5, 0, time.Local)
	got, err := GuardarChecksums(dst, cache, agora)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(cache, "CHECKSUMS-20261016-143005.sha256"); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	b, err := os.ReadFile(got)
	if err != nil || string(b) != "abc  x\n" {
		t.Fatalf("conteúdo: %q %v", b, err)
	}
}
