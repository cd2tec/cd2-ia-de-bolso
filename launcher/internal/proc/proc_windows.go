package proc

import (
	"os/exec"
	"strconv"
	"syscall"
)

const criarSemJanela = 0x08000000 // CREATE_NO_WINDOW

func ajustar(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: criarSemJanela}
}

// encerrar derruba a árvore inteira: o llamafile pode deixar filhos.
func encerrar(p *Processo) error {
	k := exec.Command("taskkill", "/PID", strconv.Itoa(p.Pid()), "/T", "/F")
	k.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: criarSemJanela}
	if err := k.Run(); err != nil {
		select {
		case <-p.morreu: // já tinha saído
			return nil
		default:
		}
		return err
	}
	return nil
}
