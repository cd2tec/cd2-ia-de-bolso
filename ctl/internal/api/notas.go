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
	b, ok := s.bancoAtual(w)
	if !ok {
		return
	}
	dados, err := s.notas.Ler(b)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Não consegui ler as notas.")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Banco notas.Banco     `json:"banco"`
		Dados json.RawMessage `json:"dados"`
	}{b, dados})
}

func (s *server) notasGravar(w http.ResponseWriter, r *http.Request) {
	b, ok := s.bancoAtual(w)
	if !ok {
		return
	}
	var in struct {
		Dados json.RawMessage `json:"dados"`
	}
	// folga de 4 KB para o envelope {"dados": ...}
	if !decodeJSONLimite(w, r, &in, notas.MaxDados+4096, "corpo: acima de 1 MB") {
		return
	}
	if len(in.Dados) > notas.MaxDados {
		fail(w, http.StatusRequestEntityTooLarge, "corpo: acima de 1 MB")
		return
	}
	err := s.notas.Gravar(b, in.Dados)
	if errors.Is(err, notas.ErrDados) {
		fail(w, http.StatusBadRequest, "dados: formato inválido")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "Não consegui gravar as notas.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
