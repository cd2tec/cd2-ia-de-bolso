//go:build !windows

package inst

import (
	"os/exec"
	"syscall"
)

// iniciarDestacado roda o programa em outro grupo de processos e não espera por ele.
func iniciarDestacado(caminho string, args ...string) error {
	c := exec.Command(caminho, args...)
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := c.Start(); err != nil {
		return err
	}
	go func() { _ = c.Wait() }() // colhe o processo quando sair, sem travar
	return nil
}
