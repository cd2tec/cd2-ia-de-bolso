package baixar

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/manifesto"
)

func conteudo(n int, semente byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7) + semente
	}
	return b
}

func escrever(t *testing.T, destino string, b []byte) {
	t.Helper()
	if err := os.WriteFile(destino, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func shaDe(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

type servidor struct {
	*httptest.Server
	gets   atomic.Int32
	ranges chan string
}

func novoServidor(t *testing.T, corpo []byte) *servidor {
	t.Helper()
	s := &servidor{ranges: make(chan string, 16)}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.gets.Add(1)
		s.ranges <- r.Header.Get("Range")
		if r.Header.Get("User-Agent") != "cd2-harness" {
			t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		http.ServeContent(w, r, "x.bin", time.Time{}, bytes.NewReader(corpo))
	}))
	t.Cleanup(s.Close)
	return s
}

func TestBaixaInteiro(t *testing.T) {
	corpo := conteudo(1<<20, 1)
	s := novoServidor(t, corpo)
	destino := filepath.Join(t.TempDir(), "a.bin")
	var ultimoFeitos, ultimoTotal int64
	err := Arquivo(context.Background(), s.Client(), s.URL, destino,
		manifesto.Arquivo{SHA256: shaDe(corpo), Bytes: int64(len(corpo))},
		func(f, tot int64) { ultimoFeitos, ultimoTotal = f, tot })
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(destino)
	if !bytes.Equal(got, corpo) {
		t.Fatal("conteúdo diferente")
	}
	if ultimoTotal != 1<<20 || ultimoFeitos != 1<<20 {
		t.Errorf("progresso final = %d/%d", ultimoFeitos, ultimoTotal)
	}
}

func TestRetomaPelaMetade(t *testing.T) {
	corpo := conteudo(1<<20, 1)
	s := novoServidor(t, corpo)
	destino := filepath.Join(t.TempDir(), "a.bin")
	escrever(t, destino, corpo[:300<<10])
	err := Arquivo(context.Background(), s.Client(), s.URL, destino,
		manifesto.Arquivo{SHA256: shaDe(corpo), Bytes: int64(len(corpo))}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r := <-s.ranges; r != "bytes=307200-" {
		t.Errorf("Range = %q", r)
	}
	got, _ := os.ReadFile(destino)
	if !bytes.Equal(got, corpo) {
		t.Fatal("conteúdo diferente")
	}
}

func TestPulaSeCompleto(t *testing.T) {
	corpo := conteudo(1<<20, 1)
	s := novoServidor(t, corpo)
	destino := filepath.Join(t.TempDir(), "a.bin")
	escrever(t, destino, corpo)
	chamadas := 0
	err := Arquivo(context.Background(), s.Client(), s.URL, destino,
		manifesto.Arquivo{SHA256: shaDe(corpo), Bytes: int64(len(corpo))}, func(int64, int64) { chamadas++ })
	if err != nil {
		t.Fatal(err)
	}
	if n := s.gets.Load(); n != 0 {
		t.Errorf("GETs = %d, esperado 0", n)
	}
	// Nada foi baixado: sem progresso, para a tela não mostrar "Baixando" à toa.
	if chamadas != 0 {
		t.Errorf("progresso chamado %d vezes num arquivo já completo", chamadas)
	}
}

func TestMetadeDeOutraVersao(t *testing.T) {
	corpo := conteudo(1<<20, 1)
	s := novoServidor(t, corpo)
	destino := filepath.Join(t.TempDir(), "a.bin")
	escrever(t, destino, conteudo(300<<10, 9))
	err := Arquivo(context.Background(), s.Client(), s.URL, destino,
		manifesto.Arquivo{SHA256: shaDe(corpo), Bytes: int64(len(corpo))}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n := s.gets.Load(); n != 2 {
		t.Errorf("GETs = %d, esperado 2", n)
	}
	got, _ := os.ReadFile(destino)
	if !bytes.Equal(got, corpo) {
		t.Fatal("conteúdo diferente")
	}
}

func TestExistenteMaiorQueOEsperado(t *testing.T) {
	corpo := conteudo(1<<20, 1)
	s := novoServidor(t, corpo)
	destino := filepath.Join(t.TempDir(), "a.bin")
	escrever(t, destino, conteudo(2<<20, 9))
	err := Arquivo(context.Background(), s.Client(), s.URL, destino,
		manifesto.Arquivo{SHA256: shaDe(corpo), Bytes: int64(len(corpo))}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n := s.gets.Load(); n != 1 {
		t.Errorf("GETs = %d, esperado 1", n)
	}
	if r := <-s.ranges; r != "" {
		t.Errorf("deveria baixar do zero, Range = %q", r)
	}
}

func TestServidorSempreErrado(t *testing.T) {
	s := novoServidor(t, conteudo(1<<20, 2))
	destino := filepath.Join(t.TempDir(), "a.bin")
	err := Arquivo(context.Background(), s.Client(), s.URL, destino,
		manifesto.Arquivo{SHA256: shaDe(conteudo(1<<20, 1)), Bytes: 1 << 20}, nil)
	if !errors.Is(err, ErrConferencia) {
		t.Fatalf("err = %v", err)
	}
	if _, e := os.Stat(destino); !errors.Is(e, fs.ErrNotExist) {
		t.Error("destino deveria ter sido apagado")
	}
	if n := s.gets.Load(); n != 2 {
		t.Errorf("GETs = %d, esperado 2", n)
	}
}

func TestSemSHAConfereTamanho(t *testing.T) {
	s := novoServidor(t, conteudo(1000, 1))
	destino := filepath.Join(t.TempDir(), "a.bin")
	err := Arquivo(context.Background(), s.Client(), s.URL, destino,
		manifesto.Arquivo{Bytes: 999}, nil)
	if !errors.Is(err, ErrConferencia) {
		t.Fatalf("err = %v", err)
	}
}

func TestCancelar(t *testing.T) {
	corpo := conteudo(1<<20, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1048576")
		w.Write(corpo[:200<<10])
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	destino := filepath.Join(t.TempDir(), "a.bin")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := Arquivo(ctx, srv.Client(), srv.URL, destino,
		manifesto.Arquivo{SHA256: shaDe(corpo), Bytes: int64(len(corpo))},
		func(f, _ int64) {
			if f > 0 {
				cancel()
			}
		})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	st, e := os.Stat(destino)
	if e != nil || st.Size() == 0 {
		t.Errorf("parcial deveria ficar: %v", e)
	}
}

func TestConferirSemRede(t *testing.T) {
	err := Conferir(filepath.Join(t.TempDir(), "nao-existe"), manifesto.Arquivo{SHA256: "ab"})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v", err)
	}
}

func TestSemSHASemBytesUsaHEAD(t *testing.T) {
	corpo := conteudo(1<<20, 1)
	s := novoServidor(t, corpo)
	destino := filepath.Join(t.TempDir(), "a.bin")
	escrever(t, destino, corpo)
	if err := Arquivo(context.Background(), s.Client(), s.URL, destino, manifesto.Arquivo{}, nil); err != nil {
		t.Fatal(err)
	}
	if n := s.gets.Load(); n != 1 {
		t.Errorf("requisições = %d, esperado só o HEAD", n)
	}
	if r := <-s.ranges; r != "" {
		t.Errorf("Range = %q", r)
	}
}

func TestSemSHASemBytesSemHEAD(t *testing.T) {
	corpo := conteudo(5000, 1)
	var gets atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		gets.Add(1)
		http.ServeContent(w, r, "x.bin", time.Time{}, bytes.NewReader(corpo))
	}))
	defer srv.Close()
	destino := filepath.Join(t.TempDir(), "a.bin")
	if err := Arquivo(context.Background(), srv.Client(), srv.URL, destino, manifesto.Arquivo{}, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(destino)
	if !bytes.Equal(got, corpo) || gets.Load() != 1 {
		t.Errorf("conteúdo igual=%v GETs=%d", bytes.Equal(got, corpo), gets.Load())
	}
}

func TestConferirSemEsperado(t *testing.T) {
	dir := t.TempDir()
	cheio := filepath.Join(dir, "cheio")
	vazio := filepath.Join(dir, "vazio")
	escrever(t, cheio, []byte("x"))
	escrever(t, vazio, nil)
	if err := Conferir(cheio, manifesto.Arquivo{}); err != nil {
		t.Errorf("não vazio: %v", err)
	}
	if err := Conferir(vazio, manifesto.Arquivo{}); !errors.Is(err, ErrConferencia) {
		t.Errorf("vazio: %v", err)
	}
}

func TestServidorIgnoraRange(t *testing.T) {
	corpo := conteudo(1<<20, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1048576")
		if r.Method != http.MethodHead {
			w.Write(corpo)
		}
	}))
	defer srv.Close()
	destino := filepath.Join(t.TempDir(), "a.bin")
	escrever(t, destino, corpo[:300<<10])
	err := Arquivo(context.Background(), srv.Client(), srv.URL, destino,
		manifesto.Arquivo{SHA256: shaDe(corpo), Bytes: int64(len(corpo))}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(destino)
	if !bytes.Equal(got, corpo) {
		t.Fatal("deveria recomeçar do zero")
	}
}

func TestStatus500EhErrHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "quebrou", http.StatusInternalServerError)
	}))
	defer srv.Close()
	err := Arquivo(context.Background(), srv.Client(), srv.URL, filepath.Join(t.TempDir(), "a.bin"),
		manifesto.Arquivo{SHA256: shaDe([]byte("x")), Bytes: 1}, nil)
	if !errors.Is(err, ErrHTTP) {
		t.Fatalf("err = %v, esperava ErrHTTP", err)
	}
}
