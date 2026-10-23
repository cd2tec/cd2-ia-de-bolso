// Package api é a porta de controle do cd2-ctl: GET /status, POST e DELETE /config e POST /reler.
// Host, Origin, chave da sessão e Content-Type são conferidos antes de qualquer coisa.
package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strconv"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/control"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/dbconf"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/notas"
)

// MaxBody é o maior corpo aceito em POST /config.
const MaxBody = 8 << 10

// Options: chave da sessão (64 hex), porta do harness (P) e porta de controle (P+3000).
type Options struct {
	Key         string
	HarnessPort int
	CtlPort     int
	// Instalacao liga as rotas /instalacao*; nil as deixa em 404.
	Instalacao Instalacao
	// Notas liga as rotas /notas*; nil as deixa em 404.
	Notas *notas.Store
	// Entendimento liga as rotas /entendimento; nil as deixa em 404.
	Entendimento *notas.Store
	// PodeImportar diz se POST /notas/importar é aceito (só na versão instalada).
	PodeImportar func() bool
}

// Backend é o que a API controla (*control.Controller).
type Backend interface {
	Status() control.Status
	Apply(dbconf.Input) (control.Status, error)
	Clear() control.Status
	// Reler relê a pasta da conexão em vigor (excel ou dbf); control.ErrNaoArquivo nos outros casos.
	Reler() (control.Status, error)
}

type server struct {
	b       Backend
	inst    Instalacao
	notas   *notas.Store
	entend  *notas.Store
	importa func() bool
	auth    []byte
	origins map[string]bool
	hosts   map[string]bool
}

// Handler monta o http.Handler da porta de controle.
func Handler(opt Options, b Backend) http.Handler {
	h, c := strconv.Itoa(opt.HarnessPort), strconv.Itoa(opt.CtlPort)
	return &server{
		b:       b,
		inst:    opt.Instalacao,
		notas:   opt.Notas,
		entend:  opt.Entendimento,
		importa: opt.PodeImportar,
		auth:    []byte("Bearer " + opt.Key),
		origins: map[string]bool{"http://127.0.0.1:" + h: true, "http://localhost:" + h: true},
		hosts:   map[string]bool{"127.0.0.1:" + c: true, "localhost:" + c: true},
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"erro": msg})
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Host: contra DNS rebinding
	if !s.hosts[r.Host] {
		fail(w, http.StatusForbidden, "host não permitido")
		return
	}
	// Origin: só a do harness; ausente só em GET (testes com curl)
	origin := r.Header.Get("Origin")
	if origin != "" {
		if !s.origins[origin] {
			fail(w, http.StatusForbidden, "origem não permitida")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	} else if r.Method != http.MethodGet {
		fail(w, http.StatusForbidden, "origem ausente")
		return
	}
	// preflight do CORS: o navegador não manda a chave nele
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE")
		w.Header().Set("Access-Control-Max-Age", "600")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), s.auth) != 1 {
		fail(w, http.StatusUnauthorized, "chave da sessão ausente ou errada")
		return
	}
	switch r.URL.Path {
	case "/status":
		if r.Method != http.MethodGet {
			notAllowed(w, "GET")
			return
		}
		writeJSON(w, http.StatusOK, s.b.Status())
	case "/config":
		switch r.Method {
		case http.MethodPost:
			s.post(w, r)
		case http.MethodDelete:
			st := s.b.Clear()
			writeJSON(w, codeFor(st), st)
		default:
			notAllowed(w, "POST, DELETE")
		}
	case "/reler":
		// sem corpo: não exige Content-Type nem lê o corpo
		if r.Method != http.MethodPost {
			notAllowed(w, "POST")
			return
		}
		st, err := s.b.Reler()
		if errors.Is(err, control.ErrNaoArquivo) {
			fail(w, http.StatusConflict, err.Error())
			return
		}
		if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, codeFor(st), st)
	case "/instalacao", "/instalacao/chave", "/instalacao/progresso", "/instalacao/abrir":
		if s.inst == nil {
			fail(w, http.StatusNotFound, "rota não encontrada")
			return
		}
		s.instalacao(w, r)
	case "/notas", "/notas/importar":
		if s.notas == nil {
			fail(w, http.StatusNotFound, "rota não encontrada")
			return
		}
		s.notasRota(w, r)
	case "/entendimento":
		if s.entend == nil {
			fail(w, http.StatusNotFound, "rota não encontrada")
			return
		}
		s.entendimentoRota(w, r)
	default:
		fail(w, http.StatusNotFound, "rota não encontrada")
	}
}

func notAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	fail(w, http.StatusMethodNotAllowed, "método não permitido")
}

// decodeJSON confere o Content-Type e lê o corpo (limite MaxBody, sem campos desconhecidos).
// Se falhar, já respondeu e devolve false.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	return decodeJSONLimite(w, r, v, MaxBody, "corpo: acima de 8 KB")
}

// decodeJSONLimite é o decodeJSON com limite e mensagem de 413 próprios.
func decodeJSONLimite(w http.ResponseWriter, r *http.Request, v any, limite int64, msg413 string) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		fail(w, http.StatusUnsupportedMediaType, "use Content-Type: application/json")
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, limite))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			fail(w, http.StatusRequestEntityTooLarge, msg413)
			return false
		}
		fail(w, http.StatusBadRequest, "corpo: JSON inválido")
		return false
	}
	return true
}

func (s *server) post(w http.ResponseWriter, r *http.Request) {
	var in dbconf.Input
	if !decodeJSON(w, r, &in) {
		return
	}
	st, err := s.b.Apply(in)
	var fe *dbconf.FieldError
	if errors.As(err, &fe) {
		fail(w, http.StatusBadRequest, fe.Error())
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, codeFor(st), st)
}

// codeFor: depois do Shutdown o controlador devolve o status com MsgClosed; vira 503.
func codeFor(st control.Status) int {
	if st.Erro == control.MsgClosed {
		return http.StatusServiceUnavailable
	}
	return http.StatusOK
}
