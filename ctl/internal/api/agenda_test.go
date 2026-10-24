package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/agenda"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/control"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/notas"
)

type agendaFalsa struct {
	ligado  bool
	rodarEr error
	espera  bool // RodarAgora só volta quando ctx acaba
	apagou  []string
	viu     []string
	perda   int64
}

func (a *agendaFalsa) Ligado() bool { return a.ligado }
func (a *agendaFalsa) Pagina(chave, id string, antes int64, n int) ([]agenda.Execucao, bool, error) {
	if id == "ruim" {
		return nil, false, agenda.ErrId
	}
	return []agenda.Execucao{{Seq: 3, Tipo: "manual"}}, true, nil
}
func (a *agendaFalsa) Contar(chave, id string) (int, error) { return 7, nil }
func (a *agendaFalsa) MarcarVisto(chave, id string) error   { a.viu = append(a.viu, id); return nil }
func (a *agendaFalsa) Apagar(chave, id string) error        { a.apagou = append(a.apagou, id); return nil }
func (a *agendaFalsa) RodarAgora(ctx context.Context, b notas.Banco, id string, perda int64) (agenda.Execucao, error) {
	a.perda = perda
	if a.espera {
		<-ctx.Done()
		return agenda.Execucao{}, ctx.Err()
	}
	return agenda.Execucao{Seq: 9, Tipo: "manual"}, a.rodarEr
}
func (a *agendaFalsa) Avisos(desde int64) ([]agenda.Aviso, int64) {
	return []agenda.Aviso{{Seq: 5, ConsultaId: "c1", Titulo: "t", Texto: "x"}}, 5
}
func (a *agendaFalsa) Novidades(b notas.Banco) []agenda.Novidade {
	return []agenda.Novidade{{ConsultaId: "c1", Nome: "Vendas", N: 2}}
}

func newAgenda(t *testing.T, st control.Status, ag Agenda) http.Handler {
	t.Helper()
	return Handler(Options{Key: key, HarnessPort: 8080, CtlPort: 11080, Agenda: ag}, &backendNotas{st: st})
}

func TestAgendaNilDa404(t *testing.T) {
	h := newAgenda(t, comBanco, nil)
	if w := do(h, "GET", "/agenda/novidades", "", nil); w.Code != 404 {
		t.Fatalf("%d", w.Code)
	}
}

func TestHistoricoChaveErrada409(t *testing.T) {
	h := newAgenda(t, comBanco, &agendaFalsa{})
	w := do(h, "GET", "/agenda/historico?chave=outra&consulta=c1", "", nil)
	if w.Code != 409 || erro(w) != "O banco conectado mudou. Abra a tela de novo." {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	w = do(h, "POST", "/agenda/visto", `{"chave":"outra","consulta":"c1"}`, nil)
	if w.Code != 409 {
		t.Fatalf("visto: %d %s", w.Code, w.Body)
	}
}

func TestHistoricoLe(t *testing.T) {
	h := newAgenda(t, comBanco, &agendaFalsa{})
	w := do(h, "GET", "/agenda/historico?chave="+chaveOk+"&consulta=c1&n=500&antes=10", "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"mais":true`) || !strings.Contains(w.Body.String(), `"total":7`) || !strings.Contains(w.Body.String(), `"seq":3`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w := do(h, "GET", "/agenda/historico?chave="+chaveOk+"&consulta=c1&n=x", "", nil); w.Code != 400 {
		t.Errorf("n inválido: %d", w.Code)
	}
}

func TestRodarPendrive403(t *testing.T) {
	ag := &agendaFalsa{ligado: false}
	h := newAgenda(t, comBanco, ag)
	w := do(h, "POST", "/agenda/rodar", `{"chave":"`+chaveOk+`","consulta":"c1","perda":0}`, nil)
	if w.Code != 403 || erro(w) != "Agendamento só na versão instalada." {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if g := do(h, "GET", "/agenda/historico?chave="+chaveOk+"&consulta=c1", "", nil); g.Code != 200 {
		t.Errorf("leitura no pendrive: %d", g.Code)
	}
	if v := do(h, "POST", "/agenda/visto", `{"chave":"`+chaveOk+`","consulta":"c1"}`, nil); v.Code != 200 || len(ag.viu) != 1 {
		t.Errorf("visto no pendrive: %d", v.Code)
	}
}

func TestRodarDevolveExecucao(t *testing.T) {
	ag := &agendaFalsa{ligado: true}
	h := newAgenda(t, comBanco, ag)
	w := do(h, "POST", "/agenda/rodar", `{"chave":"`+chaveOk+`","consulta":"c1","perda":4}`, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"seq":9`) || ag.perda != 4 {
		t.Fatalf("%d %s perda=%d", w.Code, w.Body, ag.perda)
	}
}

func TestRodarErros(t *testing.T) {
	casos := []struct {
		err  error
		code int
		msg  string
	}{
		{agenda.ErrSemConsulta, 404, "Consulta não encontrada."},
		{agenda.ErrSemPerda, 404, "Execução perdida não encontrada."},
		{agenda.ErrBancoMudou, 409, "O banco conectado mudou. Abra a tela de novo."},
		{agenda.ErrParado, 503, "O agendamento está parando."},
		{agenda.ErrId, 400, "consulta: inválida"},
	}
	for _, c := range casos {
		h := newAgenda(t, comBanco, &agendaFalsa{ligado: true, rodarEr: c.err})
		w := do(h, "POST", "/agenda/rodar", `{"chave":"`+chaveOk+`","consulta":"c1"}`, nil)
		if w.Code != c.code || erro(w) != c.msg {
			t.Errorf("%v: %d %s", c.err, w.Code, w.Body)
		}
	}
}

func TestRodarDemorouDemais504(t *testing.T) {
	old := TempoRodar
	TempoRodar = 20 * time.Millisecond
	defer func() { TempoRodar = old }()
	h := newAgenda(t, comBanco, &agendaFalsa{ligado: true, espera: true})
	w := do(h, "POST", "/agenda/rodar", `{"chave":"`+chaveOk+`","consulta":"c1"}`, nil)
	if w.Code != 504 || erro(w) != "A consulta demorou demais." {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestNovidadesSemBanco(t *testing.T) {
	h := newAgenda(t, control.Status{}, &agendaFalsa{ligado: true})
	w := do(h, "GET", "/agenda/novidades?desde=2", "", nil)
	b := w.Body.String()
	if w.Code != 200 || !strings.Contains(b, `"novidades":[]`) || !strings.Contains(b, `"total":0`) ||
		!strings.Contains(b, `"agenda":true`) || !strings.Contains(b, `"ultimo":5`) || !strings.Contains(b, `"avisos":[{`) {
		t.Fatalf("%d %s", w.Code, b)
	}
}

func TestNovidadesComBanco(t *testing.T) {
	h := newAgenda(t, comBanco, &agendaFalsa{})
	w := do(h, "GET", "/agenda/novidades", "", nil)
	b := w.Body.String()
	if w.Code != 200 || !strings.Contains(b, `"nome":"Vendas"`) || !strings.Contains(b, `"total":2`) || !strings.Contains(b, `"agenda":false`) {
		t.Fatalf("%d %s", w.Code, b)
	}
}

func TestDeleteHistorico(t *testing.T) {
	ag := &agendaFalsa{}
	h := newAgenda(t, comBanco, ag)
	w := do(h, "DELETE", "/agenda/historico?chave="+chaveOk+"&consulta=c1", "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"ok":true`) || len(ag.apagou) != 1 || ag.apagou[0] != "c1" {
		t.Fatalf("%d %s %v", w.Code, w.Body, ag.apagou)
	}
}

func TestConsultaIdInvalido400(t *testing.T) {
	h := newAgenda(t, comBanco, &agendaFalsa{ligado: true})
	for _, id := range []string{"..%2Fx", "a%20b", "ruim!"} {
		w := do(h, "GET", "/agenda/historico?chave="+chaveOk+"&consulta="+id, "", nil)
		if w.Code != 400 || erro(w) != "consulta: inválida" {
			t.Errorf("%s: %d %s", id, w.Code, w.Body)
		}
	}
	w := do(h, "POST", "/agenda/rodar", `{"chave":"`+chaveOk+`","consulta":"../x"}`, nil)
	if w.Code != 400 {
		t.Errorf("rodar: %d %s", w.Code, w.Body)
	}
	// ErrId do histórico também vira 400
	if w := do(h, "GET", "/agenda/historico?chave="+chaveOk+"&consulta=ruim", "", nil); w.Code != 400 {
		t.Errorf("ErrId: %d", w.Code)
	}
}
