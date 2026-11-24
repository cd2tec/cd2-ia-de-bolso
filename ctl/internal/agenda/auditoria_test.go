package agenda

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// História 5 (CIDB-21): a ação sugerida pelo resumo, a auditoria e as regras que impedem ação sem
// confirmação registrada.

func comAcao(tipo string) Execucao {
	e := exec("agendada")
	e.Resumo = &Resumo{Linhas: []string{"A soma foi 2.500."}, Acao: tipo, Tentativas: 1}
	return e
}

func fixarAuditoria(t *testing.T, usuario string, em time.Time) {
	t.Helper()
	u, a := usuarioAtual, agoraAud
	usuarioAtual = func() string { return usuario }
	agoraAud = func() time.Time { return em }
	t.Cleanup(func() { usuarioAtual, agoraAud = u, a })
}

func eventosDe(evs []EventoAcao) string {
	var s []string
	for _, e := range evs {
		s = append(s, e.Evento)
	}
	return strings.Join(s, ",")
}

// H5-CT01: a execução com ação sugerida grava só a proposta; sem clique, nada mais acontece.
func TestPropostaGravadaComAExecucao(t *testing.T) {
	h := novoHist(t, t.TempDir())
	e, err := h.Anexar(chaveT, "c1", comAcao("gerar_csv"))
	if err != nil {
		t.Fatal(err)
	}
	if eventosDe(e.Auditoria) != "proposta" {
		t.Fatalf("Anexar devolveu %+v", e.Auditoria)
	}
	is, _, err := h.Pagina(chaveT, "c1", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	p := is[0].Auditoria
	if len(p) != 1 || p[0].Evento != "proposta" || p[0].Acao != "gerar_csv" || p[0].Por != "IA de Bolso" || !p[0].Em.Equal(t0) {
		t.Fatalf("auditoria = %+v", p)
	}
	// Execução sem ação sugerida: nada na auditoria (História 3).
	h.Anexar(chaveT, "c1", exec("agendada"))
	is, _, _ = h.Pagina(chaveT, "c1", 0, 10)
	if is[0].Auditoria != nil {
		t.Errorf("sem ação, auditoria = %+v", is[0].Auditoria)
	}
}

// H5-CT02/03/04/07: aprovar e executar ficam registrados com quem e quando.
func TestAprovarEExecutar(t *testing.T) {
	h := novoHist(t, t.TempDir())
	e, _ := h.Anexar(chaveT, "c1", comAcao("copiar_resumo"))
	quando := time.Date(2026, 11, 24, 10, 32, 5, 0, time.UTC)
	fixarAuditoria(t, "maria", quando)
	if _, err := h.Auditar(chaveT, "c1", e.Seq, e.Em, "aprovada", "copiar_resumo", ""); err != nil {
		t.Fatal(err)
	}
	evs, err := h.Auditar(chaveT, "c1", e.Seq, e.Em, "executada", "copiar_resumo", "copiado")
	if err != nil {
		t.Fatal(err)
	}
	if eventosDe(evs) != "proposta,aprovada,executada" {
		t.Fatalf("auditoria = %+v", evs)
	}
	is, _, _ := h.Pagina(chaveT, "c1", 0, 10)
	a := is[0].Auditoria
	if eventosDe(a) != "proposta,aprovada,executada" || a[1].Por != "maria" || !a[1].Em.Equal(quando) || a[2].Detalhe != "copiado" {
		t.Fatalf("lido = %+v", a)
	}
}

// H5-CT05: recusar registra a recusa, e depois nada pode ser executado.
func TestRecusarImpedeExecutar(t *testing.T) {
	h := novoHist(t, t.TempDir())
	e, _ := h.Anexar(chaveT, "c1", comAcao("rascunho_email"))
	if _, err := h.Auditar(chaveT, "c1", e.Seq, e.Em, "recusada", "rascunho_email", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Auditar(chaveT, "c1", e.Seq, e.Em, "executada", "rascunho_email", ""); !errors.Is(err, ErrSemAprovacao) {
		t.Errorf("executar recusada: %v", err)
	}
	if _, err := h.Auditar(chaveT, "c1", e.Seq, e.Em, "aprovada", "rascunho_email", ""); !errors.Is(err, ErrDecidida) {
		t.Errorf("aprovar depois de recusar: %v", err)
	}
	is, _, _ := h.Pagina(chaveT, "c1", 0, 10)
	if eventosDe(is[0].Auditoria) != "proposta,recusada" {
		t.Errorf("auditoria = %+v", is[0].Auditoria)
	}
}

// H5-CT06: a falha da ação fica registrada com o motivo.
func TestFalhaRegistrada(t *testing.T) {
	h := novoHist(t, t.TempDir())
	e, _ := h.Anexar(chaveT, "c1", comAcao("gerar_csv"))
	h.Auditar(chaveT, "c1", e.Seq, e.Em, "aprovada", "gerar_csv", "")
	evs, err := h.Auditar(chaveT, "c1", e.Seq, e.Em, "falhou", "gerar_csv", "o resultado não tem linhas")
	if err != nil || eventosDe(evs) != "proposta,aprovada,falhou" || evs[2].Detalhe != "o resultado não tem linhas" {
		t.Fatalf("%+v, %v", evs, err)
	}
	if _, err := h.Auditar(chaveT, "c1", e.Seq, e.Em, "executada", "gerar_csv", ""); !errors.Is(err, ErrSemAprovacao) {
		t.Errorf("segundo resultado: %v", err)
	}
	longo := strings.Repeat("x", 500)
	e2, _ := h.Anexar(chaveT, "c1", comAcao("gerar_csv"))
	h.Auditar(chaveT, "c1", e2.Seq, e2.Em, "aprovada", "gerar_csv", "")
	evs, _ = h.Auditar(chaveT, "c1", e2.Seq, e2.Em, "falhou", "gerar_csv", longo)
	if n := len([]rune(evs[2].Detalhe)); n != maxDetalheAud {
		t.Errorf("detalhe com %d caracteres", n)
	}
}

// Nenhuma ação sem confirmação registrada: resultado sem aprovar, ação diferente da sugerida,
// execução sem sugestão, outra execução (horário diferente) e evento inventado são recusados.
func TestRegrasDaAuditoria(t *testing.T) {
	h := novoHist(t, t.TempDir())
	e, _ := h.Anexar(chaveT, "c1", comAcao("gerar_csv"))
	sem, _ := h.Anexar(chaveT, "c1", exec("agendada"))
	casos := []struct {
		nome         string
		seq          int64
		em           time.Time
		evento, acao string
		quer         error
	}{
		{"executar sem aprovar", e.Seq, e.Em, "executada", "gerar_csv", ErrSemAprovacao},
		{"falhou sem aprovar", e.Seq, e.Em, "falhou", "gerar_csv", ErrSemAprovacao},
		{"outra ação", e.Seq, e.Em, "aprovada", "copiar_resumo", ErrSemProposta},
		{"sem sugestão", sem.Seq, sem.Em, "aprovada", "gerar_csv", ErrSemProposta},
		{"outro horário", e.Seq, e.Em.Add(time.Second), "aprovada", "gerar_csv", ErrSemProposta},
		{"seq que não existe", 99, e.Em, "aprovada", "gerar_csv", ErrSemProposta},
		{"proposta pela rota", e.Seq, e.Em, "proposta", "gerar_csv", ErrEvento},
		{"evento inventado", e.Seq, e.Em, "enviada", "gerar_csv", ErrEvento},
	}
	for _, c := range casos {
		if _, err := h.Auditar(chaveT, "c1", c.seq, c.em, c.evento, c.acao, ""); !errors.Is(err, c.quer) {
			t.Errorf("%s: %v, quer %v", c.nome, err, c.quer)
		}
	}
	if _, err := h.Auditar(chaveT, "../x", e.Seq, e.Em, "aprovada", "gerar_csv", ""); !errors.Is(err, ErrId) {
		t.Errorf("id inválido: %v", err)
	}
	is, _, _ := h.Pagina(chaveT, "c1", 0, 10)
	if eventosDe(is[1].Auditoria) != "proposta" {
		t.Errorf("as recusas não podem gravar nada: %+v", is[1].Auditoria)
	}
}

// A auditoria não some quando o histórico é apagado, e a execução nova com o mesmo seq não herda os
// eventos da antiga.
func TestAuditoriaSobreviveAoApagar(t *testing.T) {
	dir := t.TempDir()
	h := novoHist(t, dir)
	e, _ := h.Anexar(chaveT, "c1", comAcao("gerar_csv"))
	h.Auditar(chaveT, "c1", e.Seq, e.Em, "aprovada", "gerar_csv", "")
	if err := h.Apagar(chaveT, "c1"); err != nil {
		t.Fatal(err)
	}
	nova := comAcao("gerar_csv")
	nova.Em = t0.Add(time.Hour)
	n, _ := h.Anexar(chaveT, "c1", nova)
	if n.Seq != e.Seq {
		t.Fatalf("seq %d, antes %d", n.Seq, e.Seq)
	}
	is, _, _ := h.Pagina(chaveT, "c1", 0, 10)
	if eventosDe(is[0].Auditoria) != "proposta" {
		t.Errorf("a nova herdou: %+v", is[0].Auditoria)
	}
	var total int
	abrirDireto(t, dir).QueryRow(`SELECT COUNT(*) FROM auditoria`).Scan(&total)
	if total != 3 {
		t.Errorf("linhas na auditoria = %d, quer 3 (proposta e aprovada da apagada, proposta da nova)", total)
	}
}

// O banco da versão 1 (sem auditoria) ganha a tabela sem perder as execuções.
func TestEsquemaV1GanhaAuditoria(t *testing.T) {
	dir := t.TempDir()
	h := novoHist(t, dir)
	h.Anexar(chaveT, "c1", exec("agendada"))
	h.Fechar()
	db := abrirDireto(t, dir)
	if _, err := db.Exec(`DROP TABLE auditoria; PRAGMA user_version = 1`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	h2 := novoHist(t, dir)
	e, err := h2.Anexar(chaveT, "c1", comAcao("copiar_resumo"))
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := h2.Contar(chaveT, "c1"); n != 2 || e.Seq != 2 {
		t.Errorf("execuções = %d, seq %d", n, e.Seq)
	}
	if _, err := h2.Auditar(chaveT, "c1", e.Seq, e.Em, "aprovada", "copiar_resumo", ""); err != nil {
		t.Errorf("auditar depois da migração: %v", err)
	}
	var v int
	abrirDireto(t, dir).QueryRow(`PRAGMA user_version`).Scan(&v)
	if v != 2 {
		t.Errorf("user_version = %d", v)
	}
}

func respostaComAcao(acao string, frases ...string) string {
	b, _ := json.Marshal(map[string]any{"linhas": frases, "acao": acao})
	return string(b)
}

// O agendador grava a ação sugerida e a proposta, e a notificação diz que há uma ação para decidir.
func TestAgendadorPropoeAcao(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", agendaCom(limite2000))
	f.responder(linhasCom(2500), nil)
	a := novoAgendador(t, f)
	var pedido PedidoResumo
	comResumo(t, a, func(_ context.Context, p PedidoResumo) (string, error) {
		pedido = p
		return respostaComAcao("gerar_csv", "A soma de total foi 2.500, acima de 2.000."), nil
	})
	ligarExecutor(t, a)

	a.Passo(dia(26, 8, 0))
	a.esperarFila()
	var sch struct {
		Properties map[string]struct {
			Enum []string `json:"enum"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	json.Unmarshal(pedido.Schema, &sch)
	if strings.Join(sch.Properties["acao"].Enum, ",") != "copiar_resumo,rascunho_email,gerar_csv,nenhuma" || strings.Join(sch.Required, ",") != "linhas,acao" {
		t.Fatalf("schema = %s", pedido.Schema)
	}
	e := historico(t, a, "c1")[0]
	if e.Resumo == nil || e.Resumo.Acao != "gerar_csv" || eventosDe(e.Auditoria) != "proposta" {
		t.Fatalf("execução = %+v / %+v", e.Resumo, e.Auditoria)
	}
	av, _ := a.Avisos(0)
	quer := "Rodou às 08:00 · Ação sugerida: Gerar o arquivo CSV com o resultado. Abra o histórico para aprovar ou recusar.\nA soma de total foi 2.500, acima de 2.000."
	if len(av) != 1 || av[0].Texto != quer {
		t.Fatalf("aviso = %q", av[0].Texto)
	}
}

// H5-CT08: ação fora do enum é descartada; o resumo e o aviso seguem como na História 3.
// H5-CT09: com "nenhuma", sem ação: nada na auditoria e o aviso é o da História 3.
func TestAcaoForaDoEnumOuNenhuma(t *testing.T) {
	for _, c := range []struct{ acao, fora string }{{"enviar_email", "enviar_email"}, {"nenhuma", ""}} {
		f := novoAmbiente()
		f.por("c1", "Vendas", agendaCom(limite2000))
		f.responder(linhasCom(2500), nil)
		a := novoAgendador(t, f)
		comResumo(t, a, func(context.Context, PedidoResumo) (string, error) {
			return respostaComAcao(c.acao, "A soma de total foi 2.500, acima de 2.000."), nil
		})
		ligarExecutor(t, a)
		a.Passo(dia(26, 8, 0))
		a.esperarFila()
		e := historico(t, a, "c1")[0]
		if e.Resumo == nil || len(e.Resumo.Linhas) != 1 || e.Resumo.Acao != "" || e.Resumo.AcaoFora != c.fora || e.Auditoria != nil {
			t.Fatalf("%s: execução = %+v / %+v", c.acao, e.Resumo, e.Auditoria)
		}
		av, _ := a.Avisos(0)
		if len(av) != 1 || av[0].Texto != "Rodou às 08:00 · A soma de total foi 2.500, acima de 2.000." {
			t.Fatalf("%s: aviso = %q", c.acao, av[0].Texto)
		}
	}
}
