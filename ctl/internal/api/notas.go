package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/notas"
)

// notasRota atende GET/POST /notas e POST /notas/importar.
func (s *server) notasRota(w http.ResponseWriter, r *http.Request) {
	importar := r.URL.Path == "/notas/importar"
	switch {
	case importar && r.Method != http.MethodPost:
		notAllowed(w, "POST")
	case !importar && r.Method != http.MethodGet && r.Method != http.MethodPost:
		notAllowed(w, "GET, POST")
	case r.Method == http.MethodGet:
		s.notasLer(w)
	default:
		if importar && (s.importa == nil || !s.importa()) {
			fail(w, http.StatusForbidden, "Importar só na versão instalada.")
			return
		}
		s.notasGravar(w, r)
	}
}

// bancoAtual monta a identificação do banco conectado; sem conexão já responde 409.
func (s *server) bancoAtual(w http.ResponseWriter) (notas.Banco, bool) {
	st := s.b.Status()
	if st.Motor == "" {
		fail(w, http.StatusConflict, "Nenhum banco conectado.")
		return notas.Banco{}, false
	}
	return notas.Banco{Motor: st.Motor, Host: st.Host, Porta: st.Porta, Base: st.Base, Usuario: st.Usuario, Schema: st.Schema}, true
}

func (s *server) notasLer(w http.ResponseWriter) {
	s.lerDoc(w, s.notas, "notas")
}

// lerDoc devolve {banco, chave, dados} do Store dado; "o que" entra nas mensagens de erro.
func (s *server) lerDoc(w http.ResponseWriter, st *notas.Store, oQue string) {
	b, ok := s.bancoAtual(w)
	if !ok {
		return
	}
	dados, err := st.Ler(b)
	if errors.Is(err, notas.ErrCorrompido) {
		fail(w, http.StatusInternalServerError, "O arquivo de "+oQue+" deste banco está corrompido; nada foi apagado.")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "Não consegui ler "+oQue+".")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Banco notas.Banco     `json:"banco"`
		Chave string          `json:"chave"`
		Dados json.RawMessage `json:"dados"`
	}{b, notas.Chave(b), dados})
}

func (s *server) notasGravar(w http.ResponseWriter, r *http.Request) {
	s.gravarDoc(w, r, s.notas, "as notas", "1 MB")
}

// entendimentoRota atende GET/POST /entendimento (sem rota de importação).
func (s *server) entendimentoRota(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.lerDoc(w, s.entend, "o entendimento")
	case http.MethodPost:
		s.gravarDoc(w, r, s.entend, "o entendimento", "16 MB")
	default:
		notAllowed(w, "GET, POST")
	}
}

// gravarDoc valida a chave contra o banco conectado e grava no Store dado; "limiteTxt" é o
// limite do Store em texto, para a mensagem do 413.
func (s *server) gravarDoc(w http.ResponseWriter, r *http.Request, st *notas.Store, oQue, limiteTxt string) {
	b, ok := s.bancoAtual(w)
	if !ok {
		return
	}
	// a chave vem do GET /notas: garante que a gravação é para o banco que a tela leu
	var in struct {
		Chave string          `json:"chave"`
		Dados json.RawMessage `json:"dados"`
	}
	// folga de 4 KB para o envelope {"dados": ...}
	msg413 := "corpo: acima de " + limiteTxt
	if !decodeJSONLimite(w, r, &in, int64(st.Limite())+4096, msg413) {
		return
	}
	if len(in.Dados) > st.Limite() {
		fail(w, http.StatusRequestEntityTooLarge, msg413)
		return
	}
	if in.Chave == "" {
		fail(w, http.StatusBadRequest, "chave: obrigatória")
		return
	}
	if in.Chave != notas.Chave(b) {
		fail(w, http.StatusConflict, "O banco conectado mudou. Abra a tela de novo.")
		return
	}
	err := st.Gravar(b, in.Dados)
	if errors.Is(err, notas.ErrDados) {
		fail(w, http.StatusBadRequest, "dados: formato inválido")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "Não consegui gravar "+oQue+".")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
