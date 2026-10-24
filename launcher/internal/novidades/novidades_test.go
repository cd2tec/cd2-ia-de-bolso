package novidades

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
)

// portaDe tira a porta do endereço do servidor de teste (127.0.0.1:N).
func portaDe(t *testing.T, addr string) int {
	t.Helper()
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := strconv.Atoi(p)
	return n
}

func TestPerguntarMandaChave(t *testing.T) {
	var auth, host, origin, desde, metodo, caminho string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, host, origin = r.Header.Get("Authorization"), r.Host, r.Header.Get("Origin")
		metodo, caminho, desde = r.Method, r.URL.Path, r.URL.Query().Get("desde")
		fmt.Fprint(w, `{"avisos":[{"seq":8,"consulta":"c17","titulo":"IA de Bolso · Vendas","texto":"Rodou às 12:00 · 3 linhas"}],`+
			`"ultimo":8,"novidades":[{"consulta":"c17","nome":"Vendas","n":2}],"total":2,"agenda":true}`)
	}))
	defer srv.Close()
	porta := portaDe(t, srv.Listener.Addr().String())

	e, err := Perguntar(context.Background(), porta, "segredo", 7)
	if err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer segredo" || host != "127.0.0.1:"+strconv.Itoa(porta) || origin != "" {
		t.Fatalf("auth %q host %q origin %q", auth, host, origin)
	}
	if metodo != http.MethodGet || caminho != "/agenda/novidades" || desde != "7" {
		t.Fatalf("%s %s?desde=%s", metodo, caminho, desde)
	}
	quer := Estado{
		Total:  2,
		Linhas: []Linha{{Consulta: "c17", Nome: "Vendas", N: 2}},
		Avisos: []Aviso{{Seq: 8, Consulta: "c17", Titulo: "IA de Bolso · Vendas", Texto: "Rodou às 12:00 · 3 linhas"}},
		Ultimo: 8,
		Agenda: true,
	}
	if !reflect.DeepEqual(e, quer) {
		t.Fatalf("Estado = %+v", e)
	}
}

func TestPerguntarStatusRuim(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()
	if _, err := Perguntar(context.Background(), portaDe(t, srv.Listener.Addr().String()), "x", 0); err == nil {
		t.Fatal("401 devia dar erro")
	}
}

func TestPerguntarCtlForaDoAr(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	porta := portaDe(t, l.Addr().String())
	l.Close() // ninguém escutando
	if _, err := Perguntar(context.Background(), porta, "x", 0); err == nil {
		t.Fatal("ctl fora do ar devia dar erro")
	}
}

func TestTextosMenuPlural(t *testing.T) {
	topo, linhas := TextosMenu(Estado{Total: 3, Linhas: []Linha{
		{Consulta: "a", Nome: "Flash de vendas de hoje", N: 2},
		{Consulta: "b", Nome: "Estoque", N: 1},
	}})
	if topo != "3 resultados novos" {
		t.Fatalf("topo = %q", topo)
	}
	if !reflect.DeepEqual(linhas, []string{"⏰ Flash de vendas de hoje (2)", "⏰ Estoque (1)"}) {
		t.Fatalf("linhas = %q", linhas)
	}
	if topo, _ := TextosMenu(Estado{Total: 1, Linhas: []Linha{{Consulta: "a", Nome: "X", N: 1}}}); topo != "1 resultado novo" {
		t.Fatalf("topo singular = %q", topo)
	}
	if topo, linhas := TextosMenu(Estado{}); topo != "" || len(linhas) != 0 {
		t.Fatalf("vazio: %q %q", topo, linhas)
	}
}

func TestTextosMenuCortaEm5(t *testing.T) {
	var e Estado
	for i := 0; i < 7; i++ {
		e.Linhas = append(e.Linhas, Linha{Consulta: strconv.Itoa(i), Nome: "C" + strconv.Itoa(i), N: 1})
		e.Total++
	}
	topo, linhas := TextosMenu(e)
	if topo != "7 resultados novos" || len(linhas) != MaxLinhas || linhas[4] != "⏰ C4 (1)" {
		t.Fatalf("topo %q linhas %q", topo, linhas)
	}
}

func TestParaAvisar(t *testing.T) {
	avisos := []Aviso{
		{Seq: 3, Consulta: "c1", Texto: "velho"},
		{Seq: 4, Consulta: "", Texto: "2 execuções não rodaram enquanto a IA estava fechada"},
		{Seq: 5, Consulta: "c2", Texto: "novo"},
	}
	// Na primeira pergunta só a perda: as execuções antigas já foram avisadas antes.
	if got := ParaAvisar(avisos, 0, true); !reflect.DeepEqual(got, avisos[1:2]) {
		t.Fatalf("primeira = %+v", got)
	}
	if got := ParaAvisar(avisos, 4, false); !reflect.DeepEqual(got, avisos[2:]) {
		t.Fatalf("depois = %+v", got)
	}
}
