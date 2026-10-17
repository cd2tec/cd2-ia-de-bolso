//go:build !darwin && !windows

package discos

import (
	"context"
	"errors"
	"os/exec"
)

var errNaoSuportado = errors.New("sistema não suportado")

type naoSuportado struct{}

// Padrao devolve um Sistema cujas chamadas sempre falham.
func Padrao() Sistema { return naoSuportado{} }

func ajustarProcesso(*exec.Cmd) {}

func (naoSuportado) Listar(context.Context) ([]Bruto, error) { return nil, errNaoSuportado }
func (naoSuportado) Apagar(context.Context, string, string) (string, error) {
	return "", errNaoSuportado
}
func (naoSuportado) Ejetar(context.Context, string, string) error { return errNaoSuportado }
