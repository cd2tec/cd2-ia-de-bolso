//go:build !windows

package proc

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func ajustar(*exec.Cmd) {}

// pelaShell diz se o erro do exec é o ENOEXEC de um arquivo sem "#!" e, se
// for, qual sh roda o arquivo.
func pelaShell(err error) (string, bool) {
	return "/bin/sh", errors.Is(err, syscall.ENOEXEC)
}

// encerrar manda TERM, espera EsperaTerm e então KILL.
func encerrar(p *Processo) error {
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil && err != os.ErrProcessDone {
		select {
		case <-p.morreu:
			return nil
		default:
		}
	}
	select {
	case <-p.morreu:
		return nil
	case <-time.After(EsperaTerm):
	}
	if err := p.cmd.Process.Kill(); err != nil && err != os.ErrProcessDone {
		select {
		case <-p.morreu:
			return nil
		default:
		}
		return err
	}
	return nil
}
