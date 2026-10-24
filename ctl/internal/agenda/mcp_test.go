package agenda

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type msgRPC struct {
	Method string          `json:"method"`
	ID     *int            `json:"id"`
	Params json.RawMessage `json:"params"`
}

// falso: toolbox mínimo. tratar pode responder tools/call; o resto é padrão.
type falso struct {
	mu      sync.Mutex
	metodos []string
	cabs    []http.Header
	tratar  func(w http.ResponseWriter, m msgRPC) bool
}

func (f *falso) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var m msgRPC
	json.NewDecoder(r.Body).Decode(&m)
	f.mu.Lock()
	f.metodos = append(f.metodos, m.Method)
	f.cabs = append(f.cabs, r.Header.Clone())
	f.mu.Unlock()
	switch m.Method {
	case "initialize":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Mcp-Session-Id", "sessao-1")
		w.Write([]byte(`{"jsonrpc":"2.0","id":` + itoa(*m.ID) + `,"result":{"protocolVersion":"2025-06-18"}}`))
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
	default:
		if f.tratar != nil && f.tratar(w, m) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":` + itoa(*m.ID) + `,"result":{"content":[{"type":"text","text":"ok"}]}}`))
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func (f *falso) contar(metodo string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, m := range f.metodos {
		if m == metodo {
			n++
		}
	}
	return n
}

func TestConsultarFazInitializeUmaVez(t *testing.T) {
	f := &falso{}
	s := httptest.NewServer(f)
	defer s.Close()
	c := NovoCliente(s.URL)
	for i := 0; i < 2; i++ {
		texto, err := c.Consultar(context.Background(), "select 1")
		if err != nil || texto != "ok" {
			t.Fatalf("texto=%q err=%v", texto, err)
		}
	}
	if n := f.contar("initialize"); n != 1 {
		t.Fatalf("initialize %d vezes, quer 1", n)
	}
	if n := f.contar("tools/call"); n != 2 {
		t.Fatalf("tools/call %d vezes, quer 2", n)
	}
	h := f.cabs[len(f.cabs)-1]
	if h.Get("Mcp-Session-Id") != "sessao-1" || h.Get("MCP-Protocol-Version") != "2025-06-18" {
		t.Fatalf("cabeçalhos do tools/call: %v", h)
	}
	if f.cabs[0].Get("MCP-Protocol-Version") != "" {
		t.Fatal("initialize não deve mandar MCP-Protocol-Version")
	}
	if f.metodos[1] != "notifications/initialized" || f.cabs[1].Get("MCP-Protocol-Version") != "2025-06-18" {
		t.Fatalf("notifications/initialized deve mandar MCP-Protocol-Version: %s %v", f.metodos[1], f.cabs[1])
	}
}

func TestConsultarErroDoTool(t *testing.T) {
	f := &falso{tratar: func(w http.ResponseWriter, m msgRPC) bool {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":" + itoa(*m.ID) + ",\"result\":{\"isError\":true,\"content\":[{\"type\":\"text\",\"text\":\"tabela x não existe\"}]}}\n\n"))
		return true
	}}
	s := httptest.NewServer(f)
	defer s.Close()
	texto, err := NovoCliente(s.URL).Consultar(context.Background(), "select")
	if err != nil || texto != "ERRO: tabela x não existe" {
		t.Fatalf("texto=%q err=%v", texto, err)
	}
}

func TestConsultarRefazSessaoEm404(t *testing.T) {
	var primeiro sync.Once
	f := &falso{}
	f.tratar = func(w http.ResponseWriter, m msgRPC) bool {
		falhou := false
		primeiro.Do(func() { falhou = true })
		if falhou {
			w.WriteHeader(http.StatusNotFound)
		}
		return falhou
	}
	s := httptest.NewServer(f)
	defer s.Close()
	texto, err := NovoCliente(s.URL).Consultar(context.Background(), "select 1")
	if err != nil || texto != "ok" {
		t.Fatalf("texto=%q err=%v", texto, err)
	}
	if n := f.contar("initialize"); n != 2 {
		t.Fatalf("initialize %d vezes, quer 2", n)
	}
}

func TestConsultarPrazo(t *testing.T) {
	f := &falso{tratar: func(w http.ResponseWriter, m msgRPC) bool {
		time.Sleep(300 * time.Millisecond)
		return false
	}}
	s := httptest.NewServer(f)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := NovoCliente(s.URL).Consultar(ctx, "select 1")
	if !errors.Is(err, ErrPrazo) {
		t.Fatalf("err=%v, quer ErrPrazo", err)
	}
	if strings.Contains(err.Error(), "context") {
		t.Fatal("ErrPrazo deve ser a do pacote")
	}
}

func TestConsultarSemToolboxDaErrSemConexao(t *testing.T) {
	s := httptest.NewServer(&falso{})
	url := s.URL
	s.Close() // ninguém escutando: conexão recusada
	_, err := NovoCliente(url).Consultar(context.Background(), "select 1")
	if !errors.Is(err, ErrSemConexao) {
		t.Fatalf("err = %v; quer ErrSemConexao", err)
	}
	// Prazo continua sendo ErrPrazo, não falta de conexão.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NovoCliente(url).Consultar(ctx, "select 1"); !errors.Is(err, ErrPrazo) {
		t.Fatalf("ctx cancelado: err = %v; quer ErrPrazo", err)
	}
}
