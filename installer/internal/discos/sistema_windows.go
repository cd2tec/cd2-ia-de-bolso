//go:build windows

package discos

import (
	"os/exec"
	"syscall"
)

// Padrao devolve o Sistema real deste computador.
func Padrao() Sistema { return NovoWindows(executorReal) }

// ajustarProcesso esconde a janela de console do PowerShell.
func ajustarProcesso(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
