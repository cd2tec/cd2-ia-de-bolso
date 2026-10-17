package proc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

const criarSemJanela = 0x08000000 // CREATE_NO_WINDOW

func ajustar(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: criarSemJanela}
}

// No Windows o llamafile.exe é um PE de verdade: não há sh para cair.
func pelaShell(error) (string, bool) { return "", false }

// encerrar derruba a árvore inteira: o llamafile pode deixar filhos. Usa o
// taskkill do System32 (não o do PATH) e, se ele falhar, mata só o processo.
func encerrar(p *Processo) error {
	exe := "taskkill"
	if raiz := os.Getenv("SystemRoot"); raiz != "" {
		exe = filepath.Join(raiz, "System32", "taskkill.exe")
	}
	k := exec.Command(exe, "/PID", strconv.Itoa(p.Pid()), "/T", "/F")
	k.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: criarSemJanela}
	if err := k.Run(); err != nil {
		if err := p.cmd.Process.Kill(); err != nil && err != os.ErrProcessDone {
			return err
		}
	}
	return nil
}
