package api

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/agenda"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/notas"
)

// Agenda é o que as rotas /agenda* usam do agendador e do histórico (ver o adaptador no main).
type Agenda interface {
	// Ligado diz se o agendador está rodando (só na versão instalada).
	Ligado() bool
	Pagina(chave, id string, antes int64, n int) ([]agenda.Execucao, bool, error)
	Contar(chave, id string) (int, error)
	MarcarVisto(chave, id string) error
	Apagar(chave, id string) error
	RodarAgora(ctx context.Context, b notas.Banco, id string, perdaSeq int64) (agenda.Execucao, error)
	Avisos(desde int64) ([]agenda.Aviso, int64)
	Novidades(b notas.Banco) []agenda.Novidade
}

// TempoRodar é quanto POST /agenda/rodar espera a consulta antes do 504.
var TempoRodar = 20 * time.Second

var reConsulta = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

const msgBancoMudou = "O banco conectado mudou. Abra a tela de novo."

// agendaRota atende as rotas /agenda*.
func (s *server) agendaRota(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/agenda/historico":
		switch r.Method {
		case http.MethodGet:
			s.agendaHistorico(w, r)
		case http.MethodDelete:
			s.agendaApagar(w, r)
		default:
			notAllowed(w, "GET, DELETE")
		}
	case "/agenda/rodar":
		if r.Method != http.MethodPost {
			notAllowed(w, "POST")
			return
		}
		s.agendaRodar(w, r)
	case "/agenda/visto":
		if r.Method != http.MethodPost {
			notAllowed(w, "POST")
			return
		}
		s.agendaVisto(w, r)
	case "/agenda/novidades":
		if r.Method != http.MethodGet {
			notAllowed(w, "GET")
			return
		}
		s.agendaNovidades(w, r)
	default:
		fail(w, http.StatusNotFound, "rota não encontrada")
	}
}

// agendaAlvo confere chave e consulta contra o banco conectado; já responde se algo não bate.
func (s *server) agendaAlvo(w http.ResponseWriter, chave, id string) (notas.Banco, bool) {
	b, ok := s.bancoAtual(w)
	if !ok {
		return b, false
	}
	switch {
	case chave == "":
		fail(w, http.StatusBadRequest, "chave: obrigatória")
	case chave != notas.Chave(b):
		fail(w, http.StatusConflict, msgBancoMudou)
	case !reConsulta.MatchString(id):
		fail(w, http.StatusBadRequest, "consulta: inválida")
	default:
		return b, true
	}
	return b, false
}

// agendaErro traduz os erros do agendador e do histórico.
func agendaErro(w http.ResponseWriter, err error, msgGenerica string) {
	switch {
	case errors.Is(err, agenda.ErrId):
		fail(w, http.StatusBadRequest, "consulta: inválida")
	case errors.Is(err, agenda.ErrSemConsulta):
		fail(w, http.StatusNotFound, "Consulta não encontrada.")
	case errors.Is(err, agenda.ErrSemPerda):
		fail(w, http.StatusNotFound, "Execução perdida não encontrada.")
	case errors.Is(err, agenda.ErrBancoMudou):
		fail(w, http.StatusConflict, msgBancoMudou)
	case errors.Is(err, agenda.ErrParado):
		fail(w, http.StatusServiceUnavailable, "O agendamento está parando.")
	case errors.Is(err, context.DeadlineExceeded):
		fail(w, http.StatusGatewayTimeout, "A consulta demorou demais.")
	default:
		fail(w, http.StatusInternalServerError, msgGenerica)
	}
}

func (s *server) agendaHistorico(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	b, ok := s.agendaAlvo(w, q.Get("chave"), q.Get("consulta"))
	if !ok {
		return
	}
	var antes int64
	n := 20
	if v := q.Get("antes"); v != "" {
		var err error
		if antes, err = strconv.ParseInt(v, 10, 64); err != nil || antes < 0 {
			fail(w, http.StatusBadRequest, "antes: inválido")
			return
		}
	}
	if v := q.Get("n"); v != "" {
		var err error
		if n, err = strconv.Atoi(v); err != nil || n < 1 {
			fail(w, http.StatusBadRequest, "n: inválido")
			return
		}
		n = min(n, 50)
	}
	chave, id := notas.Chave(b), q.Get("consulta")
	itens, mais, err := s.agenda.Pagina(chave, id, antes, n)
	if err != nil {
		agendaErro(w, err, "Não consegui ler o histórico.")
		return
	}
	total, err := s.agenda.Contar(chave, id)
	if err != nil {
		agendaErro(w, err, "Não consegui ler o histórico.")
		return
	}
	if itens == nil {
		itens = []agenda.Execucao{}
	}
	writeJSON(w, http.StatusOK, struct {
		Itens []agenda.Execucao `json:"itens"`
		Mais  bool              `json:"mais"`
		Total int               `json:"total"`
	}{itens, mais, total})
}

func (s *server) agendaApagar(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	b, ok := s.agendaAlvo(w, q.Get("chave"), q.Get("consulta"))
	if !ok {
		return
	}
	if err := s.agenda.Apagar(notas.Chave(b), q.Get("consulta")); err != nil {
		agendaErro(w, err, "Não consegui apagar o histórico.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *server) agendaVisto(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Chave    string `json:"chave"`
		Consulta string `json:"consulta"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	b, ok := s.agendaAlvo(w, in.Chave, in.Consulta)
	if !ok {
		return
	}
	if err := s.agenda.MarcarVisto(notas.Chave(b), in.Consulta); err != nil {
		agendaErro(w, err, "Não consegui marcar como visto.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *server) agendaRodar(w http.ResponseWriter, r *http.Request) {
	if !s.agenda.Ligado() {
		fail(w, http.StatusForbidden, "Agendamento só na versão instalada.")
		return
	}
	var in struct {
		Chave    string `json:"chave"`
		Consulta string `json:"consulta"`
		Perda    int64  `json:"perda"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	b, ok := s.agendaAlvo(w, in.Chave, in.Consulta)
	if !ok {
		return
	}
	if in.Perda < 0 {
		fail(w, http.StatusBadRequest, "perda: inválida")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), TempoRodar)
	defer cancel()
	e, err := s.agenda.RodarAgora(ctx, b, in.Consulta, in.Perda)
	if err != nil {
		agendaErro(w, err, "Não consegui rodar a consulta.")
		return
	}
	writeJSON(w, http.StatusOK, e)
}

// agendaNovidades devolve os avisos novos (seq > desde) e, com banco, as consultas com execuções
// não vistas. Sem banco não é erro: o aviso do banco vem sem novidades.
func (s *server) agendaNovidades(w http.ResponseWriter, r *http.Request) {
	var desde int64
	if v := r.URL.Query().Get("desde"); v != "" {
		var err error
		if desde, err = strconv.ParseInt(v, 10, 64); err != nil || desde < 0 {
			fail(w, http.StatusBadRequest, "desde: inválido")
			return
		}
	}
	avisos, ultimo := s.agenda.Avisos(desde)
	if avisos == nil {
		avisos = []agenda.Aviso{}
	}
	nov := []agenda.Novidade{}
	total := 0
	if st := s.b.Status(); st.Motor != "" {
		b := notas.Banco{Motor: st.Motor, Host: st.Host, Porta: st.Porta, Base: st.Base, Usuario: st.Usuario, Schema: st.Schema}
		if v := s.agenda.Novidades(b); v != nil {
			nov = v
		}
		for _, n := range nov {
			total += n.N
		}
	}
	writeJSON(w, http.StatusOK, struct {
		Avisos    []agenda.Aviso    `json:"avisos"`
		Ultimo    int64             `json:"ultimo"`
		Novidades []agenda.Novidade `json:"novidades"`
		Total     int               `json:"total"`
		Agenda    bool              `json:"agenda"`
	}{avisos, ultimo, nov, total, s.agenda.Ligado()})
}
