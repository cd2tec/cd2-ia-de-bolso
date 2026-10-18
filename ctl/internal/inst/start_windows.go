//go:build windows

package inst

import (
	"os/exec"
	"syscall"
)

const (
	createNewProcessGroup = 0x00000200
	detachedProcess       = 0x00000008
)

// iniciarDestacado roda o programa sem console e fora do grupo do cd2-ctl, sem esperar por ele.
func iniciarDestacado(caminho string, args ...string) error {
	c := exec.Command(caminho, args...)
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup | detachedProcess}
	if err := c.Start(); err != nil {
		return err
	}
	go func() { _ = c.Wait() }()
	return nil
}
