package navegador

import (
	"os/exec"
	"syscall"
)

// ajustar esconde a janela de console do rundll32.
func ajustar(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
