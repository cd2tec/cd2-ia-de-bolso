//go:build !windows

package sistema

import (
	"errors"
	"os/exec"
)

var errSemRegistro = errors.New("o registro só existe no Windows")

// registroDoSistema fora do Windows não tem onde gravar.
type registroDoSistema struct{}

func (registroDoSistema) GravarTexto(string, string, string) error  { return errSemRegistro }
func (registroDoSistema) GravarNumero(string, string, uint32) error { return errSemRegistro }
func (registroDoSistema) Existe(string, string) bool                { return false }
func (registroDoSistema) ApagarValor(string, string) error          { return errSemRegistro }
func (registroDoSistema) ApagarChave(string) error                  { return errSemRegistro }

func esconderJanela(*exec.Cmd) {}
