package api

import (
	"errors"
	"net/http"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/inst"
)

// Instalacao é o que as rotas /instalacao* controlam (*inst.Backend).
type Instalacao interface {
	Info() inst.Info
	GravarChave(chave string) error
	Instalar(areaDeTrabalho, inicio bool) error
	Progresso() inst.Progresso
	Abrir() error
}

func (s *server) instalacao(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/instalacao":
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, s.inst.Info())
		case http.MethodPost:
			s.instalar(w, r)
		default:
			notAllowed(w, "GET, POST")
		}
	case "/instalacao/chave":
		if r.Method != http.MethodPost {
			notAllowed(w, "POST")
			return
		}
		var in struct {
			Chave string `json:"chave"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		if err := s.inst.GravarChave(in.Chave); err != nil {
			if errors.Is(err, inst.ErrChaveNaoVale) {
				fail(w, http.StatusBadRequest, err.Error())
				return
			}
			if errors.Is(err, inst.ErrSoPendrive) {
				fail(w, http.StatusForbidden, err.Error())
				return
			}
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case "/instalacao/progresso":
		if r.Method != http.MethodGet {
			notAllowed(w, "GET")
			return
		}
		writeJSON(w, http.StatusOK, s.inst.Progresso())
	case "/instalacao/abrir":
		if r.Method != http.MethodPost {
			notAllowed(w, "POST")
			return
		}
		if err := s.inst.Abrir(); err != nil {
			if errors.Is(err, inst.ErrNaoInstalada) {
				fail(w, http.StatusConflict, err.Error())
				return
			}
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
	}
}

func (s *server) instalar(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AreaDeTrabalho bool `json:"areaDeTrabalho"`
		Inicio         bool `json:"inicio"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	err := s.inst.Instalar(in.AreaDeTrabalho, in.Inicio)
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
	case errors.Is(err, inst.ErrOcupada):
		fail(w, http.StatusConflict, err.Error())
	case errors.Is(err, inst.ErrSemChave), errors.Is(err, inst.ErrSoPendrive):
		fail(w, http.StatusForbidden, err.Error())
	default:
		fail(w, http.StatusInternalServerError, err.Error())
	}
}
