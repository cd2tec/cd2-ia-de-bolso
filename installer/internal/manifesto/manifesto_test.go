package manifesto

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const manifestoValido = `{
  "versao": "0.13.0",
  "zip": {"url": "https://exemplo.test/cd2.zip", "sha256": "abc", "bytes": 10},
  "llamafile": {"url": "https://exemplo.test/llamafile", "sha256": "aaa", "bytes": 20},
  "modelo": {"url": "https://exemplo.test/modelo.gguf", "sha256": "def", "bytes": 30},
  "modelo_nome": "modelo.gguf",
  "toolbox": {"versao": "1.14.0", "base": "https://exemplo.test/toolbox", "plataformas": ["windows-amd64"]},
  "ctl": {"versao": "0.6.0", "base": "https://exemplo.test/ctl"}
}`

func TestLerRecusaManifestoIncompleto(t *testing.T) {
	_, err := Ler(strings.NewReader(`{"versao":"0.13.0"}`))
	if err == nil {
		t.Fatal("esperava erro para manifesto incompleto")
	}
	if !strings.Contains(err.Error(), "zip") {
		t.Fatalf("o erro deveria citar zip: %v", err)
	}
}

// Sem SHA do llamafile, um motor antigo e menor na cache seria retomado com
// Range, ficaria com o tamanho certo e passaria na conferência corrompido.
func TestLerExigeCamposDoMotorToolboxECtl(t *testing.T) {
	casos := map[string][2]string{
		"llamafile.url":       {`"url": "https://exemplo.test/llamafile", `, ``},
		"llamafile.sha256":    {`"sha256": "aaa", `, ``},
		"toolbox.base":        {`"base": "https://exemplo.test/toolbox"`, `"base": ""`},
		"toolbox.plataformas": {`["windows-amd64"]`, `[]`},
		"ctl.base":            {`"base": "https://exemplo.test/ctl"`, `"base": ""`},
	}
	for campo, troca := range casos {
		js := strings.Replace(manifestoValido, troca[0], troca[1], 1)
		if js == manifestoValido {
			t.Fatalf("%s: a troca não achou o trecho", campo)
		}
		_, err := Ler(strings.NewReader(js))
		if err == nil || !strings.Contains(err.Error(), campo) {
			t.Errorf("%s: esperava erro citando o campo, veio %v", campo, err)
		}
	}
}

func TestLerAceitaManifestoValido(t *testing.T) {
	m, err := Ler(strings.NewReader(manifestoValido))
	if err != nil {
		t.Fatal(err)
	}
	if m.Toolbox.Versao != "1.14.0" || m.Ctl.Versao != "0.6.0" || m.Toolbox.Plataformas[0] != "windows-amd64" {
		t.Fatalf("campos aninhados mal lidos: %+v", m)
	}
}

func servidor(t *testing.T, status int, corpo string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(corpo))
	}))
}

func TestObterGuardaNaCache(t *testing.T) {
	s := servidor(t, 200, manifestoValido)
	defer s.Close()
	cache := t.TempDir()
	m, daCache, err := Obter(context.Background(), s.Client(), s.URL, cache)
	if err != nil {
		t.Fatal(err)
	}
	if daCache {
		t.Fatal("veio da rede, daCache deveria ser false")
	}
	if m.Versao != "0.13.0" {
		t.Fatalf("versao inesperada: %s", m.Versao)
	}
	if _, err := os.Stat(filepath.Join(cache, "manifesto.json")); err != nil {
		t.Fatalf("manifesto.json nao foi gravado na cache: %v", err)
	}
}

func TestObterSemRedeUsaCache(t *testing.T) {
	s := servidor(t, 200, manifestoValido)
	cache := t.TempDir()
	if _, _, err := Obter(context.Background(), s.Client(), s.URL, cache); err != nil {
		t.Fatal(err)
	}
	s.Close()
	m, daCache, err := Obter(context.Background(), &http.Client{Timeout: 2 * time.Second}, s.URL, cache)
	if err != nil {
		t.Fatal(err)
	}
	if !daCache || m.Versao != "0.13.0" {
		t.Fatalf("esperava manifesto da cache, daCache=%v m=%+v", daCache, m)
	}
}

func TestObterSemRedeSemCache(t *testing.T) {
	s := servidor(t, 200, manifestoValido)
	s.Close()
	_, _, err := Obter(context.Background(), &http.Client{Timeout: 2 * time.Second}, s.URL, t.TempDir())
	if err != ErrSemManifesto {
		t.Fatalf("esperava ErrSemManifesto, veio %v", err)
	}
}

func TestObterNaoAceitaHTTP404(t *testing.T) {
	bom := servidor(t, 200, manifestoValido)
	cache := t.TempDir()
	if _, _, err := Obter(context.Background(), bom.Client(), bom.URL, cache); err != nil {
		t.Fatal(err)
	}
	bom.Close()
	ruim := servidor(t, 404, "nao achei")
	defer ruim.Close()
	m, daCache, err := Obter(context.Background(), ruim.Client(), ruim.URL, cache)
	if err != nil {
		t.Fatal(err)
	}
	if !daCache || m.Versao != "0.13.0" {
		t.Fatalf("404 deveria cair para a cache: daCache=%v", daCache)
	}
	// Sem cache, o 404 vira ErrSemManifesto.
	if _, _, err := Obter(context.Background(), ruim.Client(), ruim.URL, t.TempDir()); err != ErrSemManifesto {
		t.Fatalf("esperava ErrSemManifesto, veio %v", err)
	}
}

func TestObterManifestoInvalidoDaRedeCaiParaCache(t *testing.T) {
	bom := servidor(t, 200, manifestoValido)
	cache := t.TempDir()
	if _, _, err := Obter(context.Background(), bom.Client(), bom.URL, cache); err != nil {
		t.Fatal(err)
	}
	bom.Close()
	ruim := servidor(t, 200, `{"versao":"9"}`)
	defer ruim.Close()
	m, daCache, err := Obter(context.Background(), ruim.Client(), ruim.URL, cache)
	if err != nil || !daCache || m.Versao != "0.13.0" {
		t.Fatalf("manifesto invalido deveria cair para a cache: %v %v", err, daCache)
	}
}
