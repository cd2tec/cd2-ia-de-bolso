package agenda

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/notas"
)

const (
	folga     = time.Minute      // horário até 1 min atrás ainda conta como "na hora"
	prazo     = 15 * time.Second // o mesmo da tela
	maxAvisos = 100
)

var (
	ErrSemConsulta = errors.New("agenda: consulta não existe")
	ErrSemPerda    = errors.New("agenda: perda não existe")
	ErrBancoMudou  = errors.New("agenda: o banco conectado mudou")
	ErrParado      = errors.New("agenda: agendador parado")
)

// mensagens são os textos de web/ask.js MENSAGENS usados no aviso quando não há frase nem erro.
var mensagens = map[string]string{
	"sql":           "Não consegui montar essa consulta.",
	"timeout":       "A consulta demorou demais e foi cancelada para proteger o banco.",
	"timeout_front": "A consulta passou de 15 segundos e o harness parou de esperar. Ela pode continuar rodando no servidor.",
	"vazio":         "A consulta não trouxe nenhuma linha.",
}

type Opcoes struct {
	Agora     func() time.Time
	Banco     func() (notas.Banco, bool)                 // banco conectado agora
	Consultas func(notas.Banco) (json.RawMessage, error) // notas.Store.Ler
	Consultar func(ctx context.Context, sql string) (string, error)
	Motor     *Motor
	Hist      *Historico
	Tick      time.Duration // 30 s em produção
}

type Aviso struct {
	Seq        int64  `json:"seq"`
	ConsultaId string `json:"consulta"`
	Titulo     string `json:"titulo"`
	Texto      string `json:"texto"`
}

type Novidade struct {
	ConsultaId string `json:"consulta"`
	Nome       string `json:"nome"`
	N          int    `json:"n"`
}

// item é uma vez na fila. resp != nil só no manual, que espera a resposta.
type item struct {
	chave, id string
	ref       time.Time
	manual    bool
	perdaSeq  int64
	ctx       context.Context
	resp      chan resposta
}

type resposta struct {
	e   Execucao
	err error
}

// Agendador decide quando as consultas rodam e roda uma por vez (fila única no ctl inteiro).
// O estado dos ticks (chave, subiu, ultimo) só é tocado pelo Passo, que roda numa goroutine só;
// mu protege a fila, as ocupadas e os avisos, que o executor e o HTTP também tocam.
type Agendador struct {
	o Opcoes

	chave  string    // banco do último tick; "" sem banco ou antes do primeiro
	subiu  bool      // já houve tick com banco desde que o processo subiu
	ultimo time.Time // último tick com este banco
	// feitos: por chave/id, o último horário já tratado (na fila, perdido ou gravado como perda);
	// evita repetir quando um tick falha no meio e o seguinte refaz.
	feitos        map[string]time.Time
	totalAbertura int // perdas da abertura somadas entre tentativas, para o aviso único

	mu       sync.Mutex
	vazia    *sync.Cond // fila vazia e nada rodando
	fila     []item
	rodando  bool
	parado   bool
	ocupadas map[string]int // chave/id na fila ou rodando
	acordar  chan struct{}
	avisos   []Aviso
	seqAviso int64
}

func NovoAgendador(o Opcoes) *Agendador {
	a := &Agendador{o: o, ocupadas: map[string]int{}, acordar: make(chan struct{}, 1), feitos: map[string]time.Time{}}
	a.vazia = sync.NewCond(&a.mu)
	return a
}

// consulta salva: id, nome e o JSON inteiro, que vai como está para o Motor.
type consulta struct {
	id, nome string
	raw      json.RawMessage
}

func (a *Agendador) consultas(b notas.Banco) ([]consulta, error) {
	doc, err := a.o.Consultas(b)
	if err != nil {
		return nil, err
	}
	var d struct {
		Consultas []json.RawMessage `json:"consultas"`
	}
	if err := json.Unmarshal(doc, &d); err != nil {
		return nil, err
	}
	out := make([]consulta, 0, len(d.Consultas))
	for _, raw := range d.Consultas {
		var c struct {
			Id   string `json:"id"`
			Nome string `json:"nome"`
		}
		if json.Unmarshal(raw, &c) == nil && c.Id != "" {
			out = append(out, consulta{c.Id, c.Nome, raw})
		}
	}
	return out, nil
}

func acharConsulta(cs []consulta, id string) (consulta, bool) {
	for _, c := range cs {
		if c.id == id {
			return c, true
		}
	}
	return consulta{}, false
}

func maisTarde(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// Rodar sobe o executor e chama Passo agora e a cada Tick, até ctx acabar.
func (a *Agendador) Rodar(ctx context.Context) {
	fim := make(chan struct{})
	go func() { a.executar(ctx); close(fim) }()
	t := time.NewTicker(a.o.Tick)
	defer t.Stop()
	a.Passo(a.o.Agora())
	for {
		select {
		case <-ctx.Done():
			<-fim
			return
		case <-t.C:
			a.Passo(a.o.Agora())
		}
	}
}

// agoraDe tira a leitura monotônica: o relógio monotônico para quando o computador dorme, e
// os horários da agenda são de parede. Toda comparação com um Agora passa por aqui.
func agoraDe(t time.Time) time.Time { return t.Round(0) }

// Passo é um tick: acha as perdas, põe na fila o que está na hora e marca a IA aberta.
// Erro de leitura, ou de gravação de uma perda, deixa o tick sem avançar: o próximo tenta de
// novo, e os horários já tratados de cada consulta (feitos) não se repetem.
func (a *Agendador) Passo(agora time.Time) {
	agora = agoraDe(agora)
	b, ok := a.o.Banco()
	if !ok {
		a.chave = ""
		_ = a.o.Hist.MarcarAbertaSemBanco(agora)
		return
	}
	chave := notas.Chave(b)
	abrindo := chave != a.chave
	if !abrindo && agora.Before(a.ultimo) {
		return // o relógio voltou
	}
	cs, err := a.consultas(b)
	if err != nil {
		return
	}
	lim := agora.Add(-folga)
	desde := lim // horários em (desde, agora] vão para a fila
	var perdaDe time.Time
	motivo := ""
	if abrindo {
		aberta, qualquer, err := a.o.Hist.Aberta(chave)
		if err != nil {
			return
		}
		if !aberta.IsZero() {
			perdaDe, desde = aberta, maisTarde(lim, aberta)
			motivo = "fechada"
			if qualquer.After(aberta) {
				motivo = "outro_banco"
			}
		}
	} else {
		perdaDe, desde, motivo = a.ultimo, maisTarde(lim, a.ultimo), "dormindo"
	}
	total, falhou := 0, false
	for _, c := range cs {
		k := chave + "/" + c.id
		feito := a.feitos[k]
		depois := func(hs []time.Time) []time.Time { // só os horários ainda não tratados
			i := 0
			for i < len(hs) && !hs[i].After(feito) {
				i++
			}
			return hs[i:]
		}
		if motivo != "" && perdaDe.Before(lim) {
			hs, err := a.o.Motor.Horarios(c.raw, perdaDe, lim)
			if hs = depois(hs); err == nil && len(hs) > 0 {
				if a.perda(chave, c.id, agora, motivo, hs[0], hs[len(hs)-1], len(hs)) != nil {
					falhou = true
					continue // nem roda: o próximo tick refaz esta consulta inteira
				}
				total += len(hs)
				feito = hs[len(hs)-1]
			}
		}
		hs, err := a.o.Motor.Horarios(c.raw, desde, agora)
		if err != nil {
			a.feitos[k] = feito
			continue
		}
		for _, h := range depois(hs) {
			if !a.enfileirar(item{chave: chave, id: c.id, ref: h}, true) &&
				a.perda(chave, c.id, agora, "anterior", h, h, 1) != nil {
				falhou = true
				break
			}
			feito = h
		}
		a.feitos[k] = feito
	}
	if abrindo && !a.subiu && total > 0 {
		a.totalAbertura += total
	}
	if falhou {
		return
	}
	if abrindo && !a.subiu && a.totalAbertura > 0 {
		texto := strconv.Itoa(a.totalAbertura) + " execuções não rodaram enquanto a IA estava fechada"
		if a.totalAbertura == 1 {
			texto = "1 execução não rodou enquanto a IA estava fechada"
		}
		a.avisar("", "IA de Bolso", texto)
	}
	a.chave, a.subiu, a.ultimo = chave, true, agora
	_ = a.o.Hist.MarcarAberta(chave, agora)
}

func (a *Agendador) perda(chave, id string, em time.Time, motivo string, de, ate time.Time, n int) error {
	_, err := a.o.Hist.Anexar(chave, id, Execucao{Tipo: "perda", Ref: ate, Em: em, Motivo: motivo, De: &de, Ate: &ate, N: n})
	return err
}

// enfileirar põe o item no fim da fila. Com exclusivo, recusa (false) se a consulta já está na
// fila ou rodando. Parado, recusa sempre.
func (a *Agendador) enfileirar(it item, exclusivo bool) bool {
	k := it.chave + "/" + it.id
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.parado || exclusivo && a.ocupadas[k] > 0 {
		return false
	}
	a.ocupadas[k]++
	a.fila = append(a.fila, it)
	select {
	case a.acordar <- struct{}{}:
	default:
	}
	return true
}

// executar é a goroutine da fila: um item por vez, sem segurar mu durante a consulta. Quando
// ctx acaba, descarta o resto da fila e responde aos manuais que esperam.
func (a *Agendador) executar(ctx context.Context) {
	for {
		a.mu.Lock()
		for len(a.fila) == 0 {
			a.mu.Unlock()
			select {
			case <-ctx.Done():
				a.parar(ctx.Err())
				return
			case <-a.acordar:
			}
			a.mu.Lock()
		}
		it := a.fila[0]
		a.fila = a.fila[1:]
		a.rodando = true
		a.mu.Unlock()

		var r resposta
		switch {
		case ctx.Err() != nil:
			r.err = ctx.Err()
		case it.ctx != nil && it.ctx.Err() != nil:
			r.err = it.ctx.Err() // quem pediu já desistiu
		default:
			r.e, r.err = a.rodar(ctx, it)
		}
		a.mu.Lock()
		a.rodando = false
		a.ocupadas[it.chave+"/"+it.id]--
		a.vazia.Broadcast()
		a.mu.Unlock()
		if it.resp != nil {
			it.resp <- r
		}
	}
}

func (a *Agendador) parar(err error) {
	a.mu.Lock()
	fila := a.fila
	a.fila, a.parado, a.ocupadas = nil, true, map[string]int{}
	a.vazia.Broadcast()
	a.mu.Unlock()
	for _, it := range fila {
		if it.resp != nil {
			it.resp <- resposta{err: err}
		}
	}
}

// esperarFila (testes) espera a fila esvaziar e o item da vez terminar.
func (a *Agendador) esperarFila() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for (len(a.fila) > 0 || a.rodando) && !a.parado {
		a.vazia.Wait()
	}
}

// reler devolve o banco conectado e a consulta, se o banco ainda é o do item e a consulta existe.
func (a *Agendador) reler(it item) (notas.Banco, consulta, error) {
	b, ok := a.o.Banco()
	if !ok || notas.Chave(b) != it.chave {
		return b, consulta{}, ErrBancoMudou
	}
	cs, err := a.consultas(b)
	if err != nil {
		return b, consulta{}, err
	}
	c, ok := acharConsulta(cs, it.id)
	if !ok {
		return b, consulta{}, ErrSemConsulta
	}
	return b, c, nil
}

// rodar relê a consulta e o banco, roda e grava. Consulta apagada ou banco trocado, antes ou
// durante a consulta: descarta. Agendada que já tem um horário mais novo vencido vira perda
// "anterior" em vez de rodar atrasada.
func (a *Agendador) rodar(ctx context.Context, it item) (Execucao, error) {
	b, c, err := a.reler(it)
	if err != nil {
		return Execucao{}, err
	}
	if !it.manual {
		agora := agoraDe(a.o.Agora())
		if hs, err := a.o.Motor.Horarios(c.raw, it.ref, agora); err == nil && len(hs) > 0 {
			return Execucao{}, a.perda(it.chave, it.id, agora, "anterior", it.ref, it.ref, 1)
		}
	}
	var res Resultado
	var valores []string
	p, err := a.o.Motor.Preparar(c.raw, it.ref, b.Motor)
	switch {
	case err != nil:
		res.Erro = err.Error()
	case p.Erro != "":
		res.Erro = p.Erro
	default:
		valores = p.Valores
		cctx, cancel := context.WithTimeout(ctx, prazo)
		texto, err := a.o.Consultar(cctx, p.SQL)
		cancel()
		switch {
		case ctx.Err() != nil:
			return Execucao{}, ctx.Err() // o ctl está fechando: não grava
		case errors.Is(err, ErrPrazo) || errors.Is(err, context.DeadlineExceeded):
			res.Motivo = "timeout_front"
		case err != nil:
			res.Erro = err.Error() // banco fora do ar é execução com erro
		default:
			if res, err = a.o.Motor.Resultado(c.raw, texto, valores); err != nil {
				res = Resultado{Erro: err.Error()}
			}
		}
	}
	if _, c, err = a.reler(it); err != nil {
		return Execucao{}, err // apagada ou banco trocado enquanto rodava
	}
	tipo := "agendada"
	if it.manual {
		tipo = "manual"
	}
	e, err := a.o.Hist.Anexar(it.chave, it.id, Execucao{Tipo: tipo, Ref: it.ref, Em: agoraDe(a.o.Agora()), Valores: valores, Resultado: &res})
	if err != nil {
		return e, err
	}
	if it.perdaSeq > 0 {
		if err := a.o.Hist.MarcarRodada(it.chave, it.id, it.perdaSeq, e.Em); err != nil {
			return e, err
		}
	}
	if !it.manual {
		a.avisar(c.id, "IA de Bolso · "+c.nome, "Rodou às "+it.ref.In(time.Local).Format("15:04")+" · "+textoDoResultado(res))
	}
	return e, nil
}

func textoDoResultado(r Resultado) string {
	switch {
	case r.Frase != "":
		return r.Frase
	case r.Ok:
		return strconv.Itoa(len(r.Todas)) + " linhas" // o resultadoSalva sempre põe frase; reserva
	case r.Erro != "":
		return r.Erro
	}
	return mensagens[r.Motivo]
}

// RodarAgora roda a consulta já, na vez dela na fila, e grava como manual (sem aviso). Com
// perdaSeq > 0, usa o último horário da perda como referência e marca a perda como rodada.
func (a *Agendador) RodarAgora(ctx context.Context, b notas.Banco, id string, perdaSeq int64) (Execucao, error) {
	chave := notas.Chave(b)
	ref := agoraDe(a.o.Agora())
	if perdaSeq > 0 {
		is, _, err := a.o.Hist.Pagina(chave, id, perdaSeq+1, 1)
		if err != nil {
			return Execucao{}, err
		}
		if len(is) == 0 || is[0].Seq != perdaSeq || is[0].Tipo != "perda" || is[0].Ate == nil {
			return Execucao{}, ErrSemPerda
		}
		ref = *is[0].Ate
	}
	it := item{chave: chave, id: id, ref: ref, manual: true, perdaSeq: perdaSeq, ctx: ctx, resp: make(chan resposta, 1)}
	if !a.enfileirar(it, false) {
		return Execucao{}, ErrParado
	}
	select {
	case r := <-it.resp:
		return r.e, r.err
	case <-ctx.Done():
		return Execucao{}, ctx.Err()
	}
}

func (a *Agendador) avisar(consultaId, titulo, texto string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.seqAviso++
	a.avisos = append(a.avisos, Aviso{Seq: a.seqAviso, ConsultaId: consultaId, Titulo: titulo, Texto: texto})
	if len(a.avisos) > maxAvisos {
		a.avisos = append([]Aviso(nil), a.avisos[len(a.avisos)-maxAvisos:]...)
	}
}

// Avisos devolve os avisos com Seq > desde (dos últimos 100) e o Seq do último.
func (a *Agendador) Avisos(desde int64) (avisos []Aviso, ultimo int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, av := range a.avisos {
		if av.Seq > desde {
			avisos = append(avisos, av)
		}
	}
	return avisos, a.seqAviso
}

// Novidades lista, na ordem das notas, as consultas com execuções não vistas.
func (a *Agendador) Novidades(b notas.Banco) []Novidade {
	cs, err := a.consultas(b)
	if err != nil {
		return nil
	}
	chave := notas.Chave(b)
	var out []Novidade
	for _, c := range cs {
		if n, err := a.o.Hist.NaoVistas(chave, c.id); err == nil && n > 0 {
			out = append(out, Novidade{ConsultaId: c.id, Nome: c.nome, N: n})
		}
	}
	return out
}
