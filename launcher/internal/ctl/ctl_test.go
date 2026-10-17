package ctl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestMain faz o papel do cd2-ctl: o próprio binário de teste é re-executado
// com CTL_FALSO (ok ou morre). No modo ok grava a chave recebida em
// CTL_CHAVE_ARQ e fica vivo; no modo morre escreve o erro e sai.
func TestMain(m *testing.M) {
	modo := os.Getenv("CTL_FALSO")
	if modo == "" {
		os.Exit(m.Run())
	}
	if modo == "morre" {
		os.Stdout.WriteString("iniciando\n")
		os.Stderr.WriteString("porta em uso\n")
		os.Exit(1)
	}
	if arq := os.Getenv("CTL_CHAVE_ARQ"); arq != "" {
		_ = os.WriteFile(arq, []byte(os.Getenv("CD2_CTL_KEY")), 0o600)
	}
	time.Sleep(time.Minute)
}

func rapido(t *testing.T) {
	t.Helper()
	i := Intervalo
	Intervalo = 10 * time.Millisecond
	t.Cleanup(func() { Intervalo = i })
}

func TestCaminhoPorPlataforma(t *testing.T) {
	casos := []struct{ raiz, goos, goarch, esperado string }{
		{`E:\`, "windows", "amd64", `E:\bin\ctl\windows-amd64\cd2-ctl.exe`},
		{"/V/CD2", "darwin", "arm64", "/V/CD2/bin/ctl/darwin-arm64/cd2-ctl"},
		{"/V/CD2", "darwin", "amd64", "/V/CD2/bin/ctl/darwin-amd64/cd2-ctl"},
		{"/V/CD2", "darwin", "386", "/V/CD2/bin/ctl/darwin-amd64/cd2-ctl"},
	}
	for _, c := range casos {
		got := Caminho(c.raiz, c.goos, c.goarch)
		if got != c.esperado {
			t.Errorf("Caminho(%q,%q,%q) = %q, esperado %q", c.raiz, c.goos, c.goarch, got, c.esperado)
		}
	}
}

func TestArgs(t *testing.T) {
	got := Args("/raiz", "/cache", 8080, 42)
	esperado := []string{"--root", "/raiz", "--cache", "/cache", "--harness-port", "8080", "--parent-pid", "42"}
	if !reflect.DeepEqual(got, esperado) {
		t.Fatalf("args = %q", got)
	}
}

func TestSubirChaveSoNoFilho(t *testing.T) {
	rapido(t)
	t.Setenv("CTL_FALSO", "ok")
	dir := t.TempDir()
	arq := filepath.Join(dir, "chave.txt")
	t.Setenv("CTL_CHAVE_ARQ", arq)
	get := func(string) (int, error) { return 401, nil }
	p, err := Subir(context.Background(), os.Args[0], nil, "segredo123", 11080, dir, "darwin", get)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Parar()
	var b []byte
	for i := 0; i < 200 && len(b) == 0; i++ {
		b, _ = os.ReadFile(arq)
		time.Sleep(10 * time.Millisecond)
	}
	if string(b) != "segredo123" {
		t.Fatalf("chave no filho = %q", b)
	}
	if os.Getenv("CD2_CTL_KEY") != "" {
		t.Fatal("a chave vazou para o ambiente do lançador")
	}
}

func TestSubir401EstaNoAr(t *testing.T) {
	rapido(t)
	t.Setenv("CTL_FALSO", "ok")
	var url string
	n := 0
	get := func(u string) (int, error) {
		url = u
		n++
		if n < 2 {
			return 0, errors.New("recusou")
		}
		return 401, nil
	}
	p, err := Subir(context.Background(), os.Args[0], nil, "k", 11080, t.TempDir(), "darwin", get)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Parar()
	if n != 2 || url != "http://127.0.0.1:11080/status" {
		t.Fatalf("n=%d url=%q", n, url)
	}
}

func TestSubirFalhaTrazFimDoLog(t *testing.T) {
	rapido(t)
	t.Setenv("CTL_FALSO", "morre")
	for _, goos := range []string{"darwin", "windows"} {
		get := func(string) (int, error) { return 0, errors.New("recusou") }
		_, err := Subir(context.Background(), os.Args[0], nil, "k", 11080, t.TempDir(), goos, get)
		if !errors.Is(err, ErrNaoSubiu) || !strings.Contains(err.Error(), "porta em uso") {
			t.Fatalf("%s: err = %v", goos, err)
		}
	}
}

func TestSubirEsgotaDezTentativasEPara(t *testing.T) {
	rapido(t)
	t.Setenv("CTL_FALSO", "ok")
	n := 0
	get := func(string) (int, error) { n++; return 503, nil }
	_, err := Subir(context.Background(), os.Args[0], nil, "k", 11080, t.TempDir(), "darwin", get)
	if !errors.Is(err, ErrNaoSubiu) || n != 10 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}

func TestFimDoLogCincoLinhas(t *testing.T) {
	f := filepath.Join(t.TempDir(), "l.log")
	_ = os.WriteFile(f, []byte("1\n2\n3\n4\n5\n6\n7\n"), 0o600)
	if got := fimDoLog(f, 5); got != "3\n4\n5\n6\n7" {
		t.Fatalf("got %q", got)
	}
}
