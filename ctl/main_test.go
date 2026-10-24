package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const testKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestVersao(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"--version"}, "", nil, &out, io.Discard); code != 0 || out.String() != "cd2-ctl dev\n" {
		t.Errorf("code %d, saída %q", code, out.String())
	}
}

func TestPastaDados(t *testing.T) {
	mac := filepath.Join("/Users/a", "Library", "Application Support", "cd2-harness")
	for _, c := range []struct{ dados, goos, home, quer string }{
		{"/x/d", "darwin", "/Users/a", "/x/d"},
		{"/x/d", "windows", `C:\u`, "/x/d"},
		{"", "darwin", "/Users/a", mac},
		{"", "windows", `C:\u`, "/c"},
		{"", "linux", "/home/a", "/c"},
		{"", "darwin", "", "/c"}, // sem HOME: fica no cache
	} {
		if got := pastaDados(c.dados, "/c", c.goos, c.home); got != c.quer {
			t.Errorf("pastaDados(%q, %s, %q) = %q; quer %q", c.dados, c.goos, c.home, got, c.quer)
		}
	}
}

func TestChaveInvalida(t *testing.T) {
	for _, k := range []string{"", "curta", strings.Repeat("z", 64)} {
		var errb bytes.Buffer
		code := run([]string{"--root", "r", "--cache", t.TempDir(), "--harness-port", "18080"}, k, nil, io.Discard, &errb)
		if code != 2 || !strings.Contains(errb.String(), "CD2_CTL_KEY") {
			t.Errorf("chave %q: code %d, %q", k, code, errb.String())
		}
	}
}

func TestArgumentosFaltando(t *testing.T) {
	var errb bytes.Buffer
	if code := run(nil, testKey, nil, io.Discard, &errb); code != 2 || !strings.Contains(errb.String(), "uso:") {
		t.Errorf("code %d, %q", code, errb.String())
	}
}

func TestPortaDeControleOcupada(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	p := l.Addr().(*net.TCPAddr).Port
	if p <= 2001 {
		t.Skip("porta efêmera baixa demais")
	}
	var errb bytes.Buffer
	code := run([]string{"--root", t.TempDir(), "--cache", t.TempDir(), "--harness-port", strconv.Itoa(p - 3000)}, testKey, nil, io.Discard, &errb)
	if code != 2 || !strings.Contains(errb.String(), "nao consegui escutar") {
		t.Errorf("code %d, %q", code, errb.String())
	}
}

func TestSobeRespondeEEncerra(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := l.Addr().(*net.TCPAddr).Port
	l.Close()
	if p <= 2001 {
		t.Skip("porta efêmera baixa demais")
	}
	stop := make(chan os.Signal, 1)
	done := make(chan int, 1)
	go func() {
		done <- run([]string{"--root", t.TempDir(), "--cache", t.TempDir(), "--harness-port", strconv.Itoa(p - 3000)}, testKey, stop, io.Discard, io.Discard)
	}()
	url := fmt.Sprintf("http://127.0.0.1:%d/status", p)
	var code int
	for i := 0; i < 50; i++ {
		if resp, err := http.Get(url); err == nil {
			code = resp.StatusCode
			resp.Body.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if code != 401 {
		t.Fatalf("sem chave: %d, quero 401", code)
	}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+testKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"estado":"sem_banco"`) {
		t.Errorf("com chave: %d %s", resp.StatusCode, body)
	}
	stop <- os.Interrupt
	select {
	case c := <-done:
		if c != 0 {
			t.Errorf("saiu com %d", c)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("não encerrou com o sinal")
	}
}

// syncBuf deixa ler o bytes.Buffer enquanto run escreve nele de outra goroutine.
type syncBuf struct {
	mu sync.Mutex
	b  *bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
