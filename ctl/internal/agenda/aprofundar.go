package agenda

import (
	"context"
	"errors"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/notas"
)

// PrazoEscolha é quanto o aviso espera o modelo escolher o aprofundamento (todas as tentativas). Passou
// disso, segue sem aprofundamento: o resumo é o da História 3. A consulta escolhida tem o prazo de
// qualquer consulta (15 s) e o resumo, o PrazoResumo; o aviso espera a soma no pior caso.
const PrazoEscolha = 15 * time.Second

// Aprofundamento é o que ficou do aprofundamento de uma execução que disparou (Fase 3, CIDB-16): a
// consulta escolhida (Id, Nome), os valores e o resultado com que rodou. Falha diz por que não houve:
// "prazo" | "modelo" | "formato" | "fora" (escolha) | "consulta" (a escolhida não rodou) |
// "sem_consulta" (as vinculadas não existem mais). Ms: escolha mais consulta.
type Aprofundamento struct {
	Id         string     `json:"id,omitempty"`
	Nome       string     `json:"nome,omitempty"`
	Valores    []string   `json:"valores,omitempty"`
	Resultado  *Resultado `json:"resultado,omitempty"`
	Falha      string     `json:"falha,omitempty"`
	Detalhe    string     `json:"detalhe,omitempty"`
	Tentativas int        `json:"tentativas"`
	Ms         int64      `json:"ms"`
}

// paraResumo é o que vai para web/resumo.js: {resultado} quando o aprofundamento rodou; senão nil, e o
// resumo é o da História 3. A consulta que não trouxe linhas entra como resultado vazio ("nenhuma linha").
func (ap *Aprofundamento) paraResumo() any {
	if ap == nil || ap.Falha != "" || ap.Resultado == nil {
		return nil
	}
	r := *ap.Resultado
	if !r.Ok && r.Motivo == "vazio" {
		r = Resultado{Ok: true, Colunas: []string{}, Todas: []map[string]any{}}
	}
	if !r.Ok {
		return nil
	}
	return map[string]any{"resultado": r}
}

// aprofundar escolhe pelo modelo uma das consultas vinculadas e a roda com o SQL salvo, no horário de
// referência da execução. nil quando não há aprofundamento (sem modelo, a condição não disparou ou nada
// vinculado: o fluxo é o da História 3) ou quando o ctl está fechando. No máximo uma consulta por execução.
func (a *Agendador) aprofundar(ctx context.Context, b notas.Banco, c consulta, ref time.Time, j *Julgamento) *Aprofundamento {
	if a.o.Resumir == nil || j == nil || !j.Disparou || !j.Aviso {
		return nil
	}
	ids, err := a.o.Motor.AprofIds(c.raw)
	if err != nil || len(ids) == 0 {
		return nil
	}
	ini := time.Now()
	ap := &Aprofundamento{}
	fim := func() *Aprofundamento { ap.Ms = time.Since(ini).Milliseconds(); return ap }
	cs, err := a.consultas(b)
	if err != nil {
		ap.Falha, ap.Detalhe = "sem_consulta", err.Error()
		return fim()
	}
	lista := make([]OpcaoAprof, len(cs))
	for i, x := range cs {
		lista[i] = OpcaoAprof{Id: x.id, Nome: x.nome}
	}
	ops, err := a.o.Motor.AprofOpcoes(c.raw, lista)
	if err != nil || len(ops) == 0 {
		ap.Falha = "sem_consulta"
		return fim()
	}
	esc, ok := a.escolher(ctx, c, j, ops, ap)
	if ctx.Err() != nil {
		return nil
	}
	if !ok {
		return fim()
	}
	ap.Id, ap.Nome = esc.Id, esc.Nome
	dc, achou := acharConsulta(cs, esc.Id)
	if !achou {
		ap.Falha = "consulta"
		return fim()
	}
	res, valores, err := a.consultar(ctx, b, dc, ref)
	if ctx.Err() != nil {
		return nil
	}
	ap.Valores, ap.Resultado = valores, &res
	switch {
	case err != nil:
		ap.Falha, ap.Detalhe = "consulta", err.Error()
	case !res.Ok && res.Motivo != "vazio":
		ap.Falha, ap.Detalhe = "consulta", res.Erro
		if ap.Detalhe == "" {
			ap.Detalhe = mensagens[res.Motivo]
		}
	}
	return fim()
}

// escolher pede a escolha ao modelo, confere e tenta de novo na recusa, até o limite de web/aprofundar.js,
// tudo dentro do PrazoEscolha. Na falha, preenche ap.Falha e devolve false.
func (a *Agendador) escolher(ctx context.Context, c consulta, j *Julgamento, ops []OpcaoAprof, ap *Aprofundamento) (Escolha, bool) {
	p, err := a.o.Motor.AprofPedido(c.raw, j, ops)
	if err != nil || p == nil {
		ap.Falha = "formato"
		if err != nil {
			ap.Detalhe = err.Error()
		}
		return Escolha{}, false
	}
	pedido := *p
	pedido.Nome, pedido.Thinking = "escolha", true // thinking só nesta decisão (CIDB-17)
	prazo := a.o.PrazoEscolha
	if prazo <= 0 {
		prazo = PrazoEscolha
	}
	ectx, cancel := context.WithTimeout(ctx, prazo)
	defer cancel()
	for {
		ap.Tentativas++
		texto, err := a.o.Resumir(ectx, pedido)
		switch {
		case ctx.Err() != nil:
			return Escolha{}, false
		case err != nil && errors.Is(ectx.Err(), context.DeadlineExceeded):
			ap.Falha, ap.Detalhe = "prazo", ""
			return Escolha{}, false
		case err != nil:
			ap.Falha, ap.Detalhe = "modelo", err.Error()
			return Escolha{}, false
		}
		e, err := a.o.Motor.AprofConferir(texto, ops)
		if err != nil {
			ap.Falha, ap.Detalhe = "formato", err.Error()
			return Escolha{}, false
		}
		if e.Ok {
			ap.Falha, ap.Detalhe = "", ""
			return e, true
		}
		ap.Falha, ap.Detalhe = e.Motivo, e.Detalhe
		msgs, max, err := a.o.Motor.AprofDeNovo(pedido.Mensagens, texto, e)
		if err != nil || ap.Tentativas >= max {
			return Escolha{}, false
		}
		pedido.Mensagens = msgs
	}
}
