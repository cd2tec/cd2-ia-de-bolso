//go:build !windows

package elevacao

import (
	"errors"
	"os/exec"
	"syscall"
)

// Elevado é sempre false fora do Windows: lá o instalador não pede senha.
func Elevado() bool { return false }

// ReabrirSemAdmin só existe no Windows.
func ReabrirSemAdmin(exe string, args []string) error {
	return errors.New("reabrir sem administrador só existe no Windows")
}

// Destacar põe o programa num grupo próprio, para ele sobreviver à saída do
// instalador.
func Destacar(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
