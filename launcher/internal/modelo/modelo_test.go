package modelo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type passo struct{ feitos, total int64 }

func TestLerProgressoAteOFim(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "m.gguf")
	if err := os.WriteFile(arq, make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	var vistos []passo
	err := Ler(context.Background(), arq, 32, func(f, tot int64) { vistos = append(vistos, passo{f, tot}) })
	if err != nil {
		t.Fatal(err)
	}
	esperado := []passo{{32, 100}, {64, 100}, {96, 100}, {100, 100}}
	if len(vistos) != len(esperado) {
		t.Fatalf("vistos = %v, esperava %v", vistos, esperado)
	}
	for i := range esperado {
		if vistos[i] != esperado[i] {
			t.Fatalf("vistos = %v, esperava %v", vistos, esperado)
		}
	}
}

func TestLerCanceladoPara(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "m.gguf")
	if err := os.WriteFile(arq, make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancela := context.WithCancel(context.Background())
	cancela()
	completo := false
	err := Ler(ctx, arq, 32, func(f, tot int64) {
		if f == tot {
			completo = true
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, esperava context.Canceled", err)
	}
	if completo {
		t.Fatal("prog chegou a feitos == total mesmo cancelado")
	}
}

func TestAcharSemModelo(t *testing.T) {
	raiz := t.TempDir()
	if err := os.Mkdir(filepath.Join(raiz, "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Achar(raiz); !errors.Is(err, ErrSemModelo) {
		t.Fatalf("err = %v, esperava ErrSemModelo", err)
	}
}

func TestAcharPrimeiroEmOrdemAlfabetica(t *testing.T) {
	raiz := t.TempDir()
	dir := filepath.Join(raiz, "models")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"b.gguf", "a.gguf", "z.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Achar(raiz)
	if err != nil || got != filepath.Join(dir, "a.gguf") {
		t.Fatalf("got = %q, err = %v", got, err)
	}
}
