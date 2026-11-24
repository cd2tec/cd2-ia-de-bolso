package agenda

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/notas"
)

// agendaCom: a cada 5 minutos, com a condição dada.
func agendaCom(cond map[string]any) map[string]any {
	return map[string]any{"ligado": true, "freq": "minutos", "a_cada": 5, "condicao": cond}
}

func linhasCom(total float64) string {
	b, _ := json.Marshal([]map[string]any{{"loja": "A", "total": total}})
	return string(b)
}

func (f *ambiente) responder(texto string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.consultar = func(context.Context, string) (string, error) { return texto, err }
}

func naoVistas(t *testing.T, a *Agendador, id string) int {
	t.Helper()
	n, err := a.o.Hist.NaoVistas(notas.Chave(bancoT), id)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestTemCondicao(t *testing.T) {
	for raw, quer := range map[string]bool{
		`{"id":"c"}`:                                 false,
		`{"agenda":{"freq":"diario"}}`:               false,
		`{"agenda":{"condicao":null}}`:               false,
		`{"agenda":{"condicao":{"coluna":"total"}}}`: true,
		`não é json`:                                 false,
	} {
		if got := TemCondicao(json.RawMessage(raw)); got != quer {
			t.Errorf("TemCondicao(%s) = %v", raw, got)
		}
	}
}

func TestCondicaoLimiteSoAvisaQuandoDispara(t *testing.T) {
	f := novoAmbiente() // linhasT: soma de total = 1244,5
	f.por("c1", "Vendas", agendaCom(map[string]any{"coluna": "total", "agregacao": "soma", "operador": "maior", "valor": 2000}))
	a := novoAgendador(t, f)
	ligarExecutor(t, a)

	a.Passo(dia(26, 8, 0))
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 1 || is[0].Tipo != "agendada" || is[0].Julgamento == nil {
		t.Fatalf("histórico = %+v", is)
	}
	j := is[0].Julgamento
	if !j.Avaliada || j.Disparou || j.Aviso || j.Valor == nil || *j.Valor != 1244.5 {
		t.Fatalf("julgamento = %+v", j)
	}
	if is[0].Novo || naoVistas(t, a, "c1") != 0 {
		t.Errorf("execução sem aviso contou como novidade: novo=%v", is[0].Novo)
	}
	if av, _ := a.Avisos(0); len(av) != 0 {
		t.Errorf("avisou sem disparar: %+v", av)
	}

	f.responder(linhasCom(2500), nil)
	a.Passo(dia(26, 8, 5))
	a.esperarFila()
	is = historico(t, a, "c1")
	if len(is) != 2 || !is[0].Julgamento.Disparou || !is[0].Julgamento.Aviso || !is[0].Novo {
		t.Fatalf("histórico = %+v", is)
	}
	if n := naoVistas(t, a, "c1"); n != 1 {
		t.Errorf("não vistas = %d, quero 1", n)
	}
	av, _ := a.Avisos(0)
	if len(av) != 1 || av[0].Texto != "Rodou às 08:05 · A soma de total = 2.500, acima de 2.000." {
		t.Fatalf("avisos = %+v", av)
	}
}

func TestCondicaoVariacaoContraAnterior(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", agendaCom(map[string]any{"coluna": "total", "agregacao": "primeira", "operador": "caiu", "valor": 20, "base": "anterior"}))
	a := novoAgendador(t, f)
	ligarExecutor(t, a)

	f.responder(linhasCom(100), nil)
	a.Passo(dia(26, 8, 0))
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 1 || is[0].Julgamento == nil || is[0].Julgamento.Motivo != "sem_base" || is[0].Julgamento.Aviso {
		t.Fatalf("primeira execução, sem base: %+v", is)
	}

	// Rodar agora (manual) com outro valor: ganha julgamento, não avisa e não vira base.
	f.responder(linhasCom(1), nil)
	if _, err := a.RodarAgora(context.Background(), bancoT, "c1", 0); err != nil {
		t.Fatal(err)
	}

	f.responder(linhasCom(70), nil)
	a.Passo(dia(26, 8, 5))
	a.esperarFila()
	is = historico(t, a, "c1")
	if len(is) != 3 || is[1].Tipo != "manual" || is[1].Julgamento == nil {
		t.Fatalf("histórico = %+v", is)
	}
	j := is[0].Julgamento
	if j == nil || !j.Disparou || j.Base == nil || *j.Base != 100 || j.Variacao == nil || *j.Variacao != -30 {
		t.Fatalf("julgamento = %+v", j)
	}
	av, _ := a.Avisos(0)
	if len(av) != 1 || !strings.Contains(av[0].Texto, "caiu 30% (100 para 70) em relação à execução anterior") {
		t.Fatalf("avisos = %+v", av)
	}
}

func TestCondicaoErroDoBancoAindaAvisa(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", agendaCom(map[string]any{"coluna": "total", "agregacao": "soma", "operador": "maior", "valor": 1}))
	f.responder("", errors.New("connection refused"))
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	a.Passo(dia(26, 8, 0))
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 1 || is[0].Julgamento == nil || is[0].Julgamento.Motivo != "erro" || !is[0].Julgamento.Aviso {
		t.Fatalf("histórico = %+v", is)
	}
	av, _ := a.Avisos(0)
	if len(av) != 1 || !strings.Contains(av[0].Texto, "connection refused") {
		t.Fatalf("avisos = %+v", av)
	}
}

func TestCondicaoResultadoVazioNaoAvisa(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", agendaCom(map[string]any{"coluna": "total", "agregacao": "soma", "operador": "maior", "valor": 1}))
	f.responder("[]", nil)
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	a.Passo(dia(26, 8, 0))
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 1 || is[0].Julgamento == nil || is[0].Julgamento.Motivo != "vazio" || is[0].Julgamento.Aviso {
		t.Fatalf("histórico = %+v", is)
	}
	if av, _ := a.Avisos(0); len(av) != 0 {
		t.Errorf("avisos = %+v", av)
	}
}

func TestSemCondicaoContinuaComoAntes(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", cada5)
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	a.Passo(dia(26, 8, 0))
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 1 || is[0].Julgamento != nil || !is[0].Novo {
		t.Fatalf("histórico = %+v", is)
	}
	if av, _ := a.Avisos(0); len(av) != 1 {
		t.Errorf("avisos = %+v", av)
	}
}

func TestAnterioresSoAgendadasQueDeramCerto(t *testing.T) {
	h := novoHist(t, t.TempDir())
	chave := notas.Chave(bancoT)
	ok := func(v float64) *Resultado {
		return &Resultado{Ok: true, Todas: []map[string]any{{"total": v}}, Linhas: []map[string]any{{"total": v}}}
	}
	for _, e := range []Execucao{
		{Tipo: "agendada", Resultado: ok(1)},
		{Tipo: "manual", Resultado: ok(2)},
		{Tipo: "agendada", Resultado: &Resultado{Motivo: "vazio"}},
		{Tipo: "perda", Motivo: "fechada"},
		{Tipo: "agendada", Resultado: ok(3)},
	} {
		if _, err := h.Anexar(chave, "c1", e); err != nil {
			t.Fatal(err)
		}
	}
	ants, err := h.Anteriores(chave, "c1", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(ants) != 2 || ants[0][0]["total"] != 3.0 || ants[1][0]["total"] != 1.0 {
		t.Fatalf("anteriores = %v", ants)
	}
	if ants, _ := h.Anteriores(chave, "c1", 1); len(ants) != 1 {
		t.Errorf("limite n=1: %v", ants)
	}
}
