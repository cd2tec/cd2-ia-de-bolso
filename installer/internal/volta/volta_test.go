package volta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// chaveDoVetor lê a chave válida do vetor compartilhado com o Worker.
func chaveDoVetor(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../../instalacao/testdata/vetor.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Chave string `json:"chave"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v.Chave
}

func ouvir(t *testing.T) *Servidor {
	t.Helper()
	s, err := Ouvir(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Fechar)
	return s
}

// pedir faz um GET no servidor, com o Host informado (vazio = o normal).
func pedir(t *testing.T, s *Servidor, caminho, host string) (int, string, http.Header) {
	t.Helper()
	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d%s", s.Porta(), caminho), nil)
	if err != nil {
		t.Fatal(err)
	}
	if host != "" {
		req.Host = host
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), resp.Header
}

func nadaNoCanal(t *testing.T, s *Servidor) {
	t.Helper()
	select {
	case c := <-s.Chaves():
		t.Fatalf("chave inesperada no canal: %q", c)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestVoltaEntregaChaveNormalizada(t *testing.T) {
	s := ouvir(t)
	limpa := chaveDoVetor(t)
	suja := "  " + strings.ToLower(limpa[:40]) + "\n" + limpa[40:80] + " - " + limpa[80:] + "\t"

	cod, corpo, h := pedir(t, s, "/?chave="+url.QueryEscape(suja), "")
	if cod != 200 {
		t.Fatalf("status %d, corpo %q", cod, corpo)
	}
	if !strings.Contains(corpo, "Pronto, a chave chegou. Pode fechar esta página e voltar para o instalador.") {
		t.Fatalf("corpo inesperado: %q", corpo)
	}
	if h.Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control %q", h.Get("Cache-Control"))
	}
	if !strings.Contains(h.Get("Content-Type"), "charset=utf-8") {
		t.Fatalf("Content-Type %q", h.Get("Content-Type"))
	}
	select {
	case c := <-s.Chaves():
		if c != limpa {
			t.Fatalf("chave %q, esperava %q", c, limpa)
		}
	case <-time.After(time.Second):
		t.Fatal("a chave não chegou no canal")
	}

	// entrega uma vez só
	if cod, _, _ := pedir(t, s, "/?chave="+url.QueryEscape(limpa), ""); cod != 200 {
		t.Fatalf("segunda volta: status %d", cod)
	}
	nadaNoCanal(t, s)
}

func TestVoltaIgnoraChaveRuim(t *testing.T) {
	s := ouvir(t)
	casos := []struct {
		caminho string
		cod     int
	}{
		{"/?chave=abc", 400},
		{"/", 400},
		{"/outro", 404},
	}
	for _, c := range casos {
		cod, corpo, h := pedir(t, s, c.caminho, "")
		if cod != c.cod {
			t.Fatalf("%s: status %d, esperava %d", c.caminho, cod, c.cod)
		}
		if c.cod == 400 && !strings.Contains(corpo, "Esta chave não está completa. Copie a chave inteira da página do pagamento e cole no instalador.") {
			t.Fatalf("%s: corpo inesperado: %q", c.caminho, corpo)
		}
		if h.Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: Cache-Control %q", c.caminho, h.Get("Cache-Control"))
		}
	}
	nadaNoCanal(t, s)
}

func TestVoltaRecusaOutroHost(t *testing.T) {
	s := ouvir(t)
	limpa := chaveDoVetor(t)
	for _, host := range []string{"evil.example:" + fmt.Sprint(s.Porta()), "127.0.0.1:1", "localhost"} {
		cod, _, _ := pedir(t, s, "/?chave="+url.QueryEscape(limpa), host)
		if cod != http.StatusForbidden {
			t.Fatalf("host %q: status %d", host, cod)
		}
	}
	nadaNoCanal(t, s)

	// localhost na porta certa vale
	cod, _, _ := pedir(t, s, "/?chave="+url.QueryEscape(limpa), fmt.Sprintf("localhost:%d", s.Porta()))
	if cod != 200 {
		t.Fatalf("localhost: status %d", cod)
	}
}

// Quem recebe a chave fecha o servidor na hora; a página "Pronto" tem que
// chegar inteira mesmo assim.
func TestVoltaFecharNaoCortaAPagina(t *testing.T) {
	limpa := chaveDoVetor(t)
	for i := 0; i < 20; i++ {
		s, err := Ouvir(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			<-s.Chaves()
			s.Fechar()
		}()
		cod, corpo, _ := pedir(t, s, "/?chave="+url.QueryEscape(limpa), "")
		if cod != 200 || !strings.Contains(corpo, textoPronto) || !strings.Contains(corpo, "</html>") {
			t.Fatalf("rodada %d: status %d, corpo %q", i, cod, corpo)
		}
		s.Fechar()
	}
}
