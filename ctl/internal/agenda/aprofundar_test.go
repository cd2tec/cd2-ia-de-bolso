package agenda

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// Aprofundamento condicional (Fase 3, CIDB-16). Os casos seguem os da subtarefa de testes (CIDB-20).

const (
	sqlPrincipal = "SELECT loja, total FROM vendas"
	sqlLoja      = "SELECT loja, faturamento FROM por_loja"
	sqlDep       = "SELECT departamento, faturamento FROM por_departamento"
	sqlForn      = "SELECT fornecedor, faturamento FROM por_fornecedor"
	linhasLoja   = `[{"loja":"Loja 04 - Valinhos","faturamento":777}]`
)

// porSQL põe uma consulta salva com o SQL dado (as de aprofundamento não precisam de agenda).
func (f *ambiente) porSQL(id, nome, sql string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.consultas = append(f.consultas, map[string]any{"id": id, "nome": nome, "sql": sql, "visual": "tabela", "params": []any{}, "criada": "2026-10-01"})
}

// ambienteAprof: c1 com a condição que dispara (soma > 2000 com 2500) e os aprofundamentos dados; as
// três consultas de aprofundamento salvas. sqls guarda o SQL de cada consulta que rodou.
func ambienteAprof(t *testing.T, aprofs []string, errLoja error) (*ambiente, *[]string) {
	t.Helper()
	f := novoAmbiente()
	ag := agendaCom(limite2000)
	if aprofs != nil {
		ag["aprofundamentos"] = aprofs
	}
	f.por("c1", "Vendas", ag)
	f.porSQL("loja", "Faturamento por loja", sqlLoja)
	f.porSQL("dep", "Faturamento por departamento", sqlDep)
	f.porSQL("forn", "Faturamento por fornecedor", sqlForn)
	var mu sync.Mutex
	sqls := &[]string{}
	f.consultar = func(_ context.Context, sql string) (string, error) {
		mu.Lock()
		*sqls = append(*sqls, sql)
		mu.Unlock()
		switch {
		case strings.Contains(sql, "por_loja"):
			return linhasLoja, errLoja
		case strings.Contains(sql, "por_"):
			return `[{"x":"y","faturamento":1}]`, nil
		}
		return linhasCom(2500), nil
	}
	return f, sqls
}

func escolha(nome string) string {
	b, _ := json.Marshal(map[string]any{"escolha": nome})
	return string(b)
}

// modeloFalso responde a escolha com esc e o resumo com frases; guarda os pedidos.
type modeloFalso struct {
	mu       sync.Mutex
	escolhas []PedidoResumo
	resumos  []PedidoResumo
}

func (m *modeloFalso) ligar(t *testing.T, a *Agendador, esc func(ctx context.Context, p PedidoResumo) (string, error), frases ...string) {
	t.Helper()
	a.o.Resumir = func(ctx context.Context, p PedidoResumo) (string, error) {
		m.mu.Lock()
		if p.Nome == "escolha" {
			m.escolhas = append(m.escolhas, p)
			m.mu.Unlock()
			return esc(ctx, p)
		}
		m.resumos = append(m.resumos, p)
		m.mu.Unlock()
		return respostaModelo(frases...), nil
	}
}

func fixo(s string) func(context.Context, PedidoResumo) (string, error) {
	return func(context.Context, PedidoResumo) (string, error) { return s, nil }
}

func rodarUma(t *testing.T, a *Agendador) Execucao {
	t.Helper()
	ligarExecutor(t, a)
	a.Passo(dia(26, 8, 0))
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 1 {
		t.Fatalf("histórico = %+v", is)
	}
	return is[0]
}

// H4-CT01, CT08, CT09: três vinculadas, o modelo escolhe uma (enum), ela roda com o SQL salvo e entra no
// resumo; uma consulta de aprofundamento só.
func TestAprofundamentoEscolhidoRodaEEntraNoResumo(t *testing.T) {
	f, sqls := ambienteAprof(t, []string{"loja", "dep", "forn"}, nil)
	a := novoAgendador(t, f)
	m := &modeloFalso{}
	m.ligar(t, a, fixo(escolha("Faturamento por loja")), "A soma de total foi 2.500, acima de 2.000.", "A Loja 04 - Valinhos faturou 777.")
	e := rodarUma(t, a)

	if len(m.escolhas) != 1 || len(m.resumos) != 1 {
		t.Fatalf("escolhas = %d, resumos = %d", len(m.escolhas), len(m.resumos))
	}
	p := m.escolhas[0]
	if !p.Thinking || m.resumos[0].Thinking {
		t.Fatalf("thinking: escolha %v, resumo %v", p.Thinking, m.resumos[0].Thinking)
	}
	var sch struct {
		Properties struct {
			Escolha struct {
				Enum []string `json:"enum"`
			} `json:"escolha"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(p.Schema, &sch); err != nil || strings.Join(sch.Properties.Escolha.Enum, "|") != "Faturamento por loja|Faturamento por departamento|Faturamento por fornecedor" {
		t.Fatalf("schema = %s", p.Schema)
	}
	if len(*sqls) != 2 || (*sqls)[0] != sqlPrincipal || (*sqls)[1] != sqlLoja {
		t.Fatalf("SQLs que rodaram = %q (quer o principal e o salvo da escolhida, um só)", *sqls)
	}
	ap := e.Aprofundamento
	if ap == nil || ap.Falha != "" || ap.Id != "loja" || ap.Nome != "Faturamento por loja" || ap.Tentativas != 1 || ap.Resultado == nil || !ap.Resultado.Ok {
		t.Fatalf("aprofundamento = %+v", ap)
	}
	u := m.resumos[0].Mensagens[1]["content"]
	if !strings.Contains(u, "Aprofundamento (resultado de outra consulta") || !strings.Contains(u, "loja: Loja 04 - Valinhos; faturamento: 777") {
		t.Fatalf("pedido do resumo sem o aprofundamento:\n%s", u)
	}
	if e.Resumo == nil || len(e.Resumo.Linhas) != 2 {
		t.Fatalf("resumo = %+v (o 777 do aprofundamento vale na conferência)", e.Resumo)
	}
	av, _ := a.Avisos(0)
	if len(av) != 1 || !strings.HasSuffix(av[0].Texto, "A Loja 04 - Valinhos faturou 777.") {
		t.Fatalf("avisos = %+v", av)
	}
}

// H4-CT02: sem vinculadas, o fluxo é o da História 3 (sem chamada de escolha, pedido igual).
func TestSemAprofundamentoEhAHistoria3(t *testing.T) {
	f, sqls := ambienteAprof(t, nil, nil)
	a := novoAgendador(t, f)
	m := &modeloFalso{}
	m.ligar(t, a, fixo(escolha("x")), "A soma de total foi 2.500, acima de 2.000.")
	e := rodarUma(t, a)
	if len(m.escolhas) != 0 || len(m.resumos) != 1 || e.Aprofundamento != nil || len(*sqls) != 1 {
		t.Fatalf("escolhas = %d resumos = %d aprofundamento = %+v sqls = %q", len(m.escolhas), len(m.resumos), e.Aprofundamento, *sqls)
	}
	if strings.Contains(m.resumos[0].Mensagens[1]["content"], "Aprofundamento") || e.Resumo == nil || len(e.Resumo.Linhas) != 1 {
		t.Fatalf("resumo = %+v", e.Resumo)
	}
}

// H4-CT04: resposta fora do enum, recusada; depois do limite, segue sem aprofundamento (resumo da H3).
func TestEscolhaForaDaListaSegueSemAprofundamento(t *testing.T) {
	f, sqls := ambienteAprof(t, []string{"loja", "dep"}, nil)
	a := novoAgendador(t, f)
	m := &modeloFalso{}
	m.ligar(t, a, fixo(escolha("Faturamento por fornecedor")), "A soma de total foi 2.500, acima de 2.000.")
	e := rodarUma(t, a)
	ap := e.Aprofundamento
	if ap == nil || ap.Falha != "fora" || ap.Tentativas != 2 || ap.Id != "" || len(m.escolhas) != 2 {
		t.Fatalf("aprofundamento = %+v, escolhas = %d", ap, len(m.escolhas))
	}
	if !strings.Contains(m.escolhas[1].Mensagens[3]["content"], "não é uma das consultas da lista") {
		t.Fatalf("a 2ª tentativa não levou o motivo: %+v", m.escolhas[1].Mensagens)
	}
	if len(*sqls) != 1 || e.Resumo == nil || len(e.Resumo.Linhas) != 1 || strings.Contains(m.resumos[0].Mensagens[1]["content"], "Aprofundamento") {
		t.Fatalf("sqls = %q resumo = %+v", *sqls, e.Resumo)
	}
	if av, _ := a.Avisos(0); len(av) != 1 {
		t.Fatalf("avisos = %+v", av)
	}
}

// H4-CT05: a consulta escolhida falha; o aviso segue com o resumo sem aprofundamento.
func TestConsultaDoAprofundamentoFalhaNaoBloqueia(t *testing.T) {
	f, _ := ambienteAprof(t, []string{"loja"}, errors.New("relation por_loja does not exist"))
	a := novoAgendador(t, f)
	m := &modeloFalso{}
	m.ligar(t, a, fixo(escolha("Faturamento por loja")), "A soma de total foi 2.500, acima de 2.000.")
	e := rodarUma(t, a)
	ap := e.Aprofundamento
	if ap == nil || ap.Falha != "consulta" || ap.Nome != "Faturamento por loja" || !strings.Contains(ap.Detalhe, "por_loja") {
		t.Fatalf("aprofundamento = %+v", ap)
	}
	if e.Resumo == nil || len(e.Resumo.Linhas) != 1 || strings.Contains(m.resumos[0].Mensagens[1]["content"], "Aprofundamento") {
		t.Fatalf("resumo = %+v", e.Resumo)
	}
	if av, _ := a.Avisos(0); len(av) != 1 {
		t.Fatalf("avisos = %+v", av)
	}
}

// H4-CT06: o modelo passa do prazo na escolha; o aviso não fica preso e o resumo da H3 segue.
func TestPrazoDaEscolhaNaoBloqueia(t *testing.T) {
	f, sqls := ambienteAprof(t, []string{"loja", "dep"}, nil)
	a := novoAgendador(t, f)
	a.o.PrazoEscolha = 50 * time.Millisecond
	m := &modeloFalso{}
	m.ligar(t, a, func(ctx context.Context, _ PedidoResumo) (string, error) { <-ctx.Done(); return "", ctx.Err() }, "A soma de total foi 2.500, acima de 2.000.")
	e := rodarUma(t, a)
	if ap := e.Aprofundamento; ap == nil || ap.Falha != "prazo" || ap.Tentativas != 1 {
		t.Fatalf("aprofundamento = %+v", ap)
	}
	if len(*sqls) != 1 || e.Resumo == nil || len(e.Resumo.Linhas) != 1 {
		t.Fatalf("sqls = %q resumo = %+v", *sqls, e.Resumo)
	}
	if av, _ := a.Avisos(0); len(av) != 1 {
		t.Fatalf("avisos = %+v", av)
	}
}

// H4-CT07: a condição não dispara: nem modelo nem aprofundamento.
func TestSemDisparoSemAprofundamento(t *testing.T) {
	f, sqls := ambienteAprof(t, []string{"loja", "dep"}, nil)
	f.consultar = func(_ context.Context, sql string) (string, error) {
		*sqls = append(*sqls, sql)
		return linhasCom(100), nil
	}
	a := novoAgendador(t, f)
	m := &modeloFalso{}
	m.ligar(t, a, fixo(escolha("Faturamento por loja")), "x")
	e := rodarUma(t, a)
	if len(m.escolhas)+len(m.resumos) != 0 || e.Aprofundamento != nil || len(*sqls) != 1 {
		t.Fatalf("chamadas = %d aprofundamento = %+v sqls = %q", len(m.escolhas)+len(m.resumos), e.Aprofundamento, *sqls)
	}
}

// Vinculadas que não existem mais: sem escolha, falha registrada, resumo da H3.
func TestAprofundamentoApagado(t *testing.T) {
	f, _ := ambienteAprof(t, []string{"sumiu"}, nil)
	a := novoAgendador(t, f)
	m := &modeloFalso{}
	m.ligar(t, a, fixo(escolha("x")), "A soma de total foi 2.500, acima de 2.000.")
	e := rodarUma(t, a)
	if ap := e.Aprofundamento; ap == nil || ap.Falha != "sem_consulta" || len(m.escolhas) != 0 {
		t.Fatalf("aprofundamento = %+v escolhas = %d", ap, len(m.escolhas))
	}
	if e.Resumo == nil || len(e.Resumo.Linhas) != 1 {
		t.Fatalf("resumo = %+v", e.Resumo)
	}
}

// "Rodar agora" (manual) não chama o modelo, então não aprofunda.
func TestRodarAgoraNaoAprofunda(t *testing.T) {
	f, sqls := ambienteAprof(t, []string{"loja"}, nil)
	a := novoAgendador(t, f)
	m := &modeloFalso{}
	m.ligar(t, a, fixo(escolha("Faturamento por loja")), "x")
	ligarExecutor(t, a)
	e, err := a.RodarAgora(context.Background(), bancoT, "c1", 0)
	if err != nil || e.Aprofundamento != nil || len(m.escolhas)+len(m.resumos) != 0 || len(*sqls) != 1 {
		t.Fatalf("err = %v aprofundamento = %+v chamadas = %d sqls = %q", err, e.Aprofundamento, len(m.escolhas)+len(m.resumos), *sqls)
	}
}
