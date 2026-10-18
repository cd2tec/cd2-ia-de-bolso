package main

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// soltar desliga o programa deste processo e do console dele.
func soltar(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS}
}

// processoVivo diz se o pid ainda está rodando.
func processoVivo(pid int) bool {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	ev, err := windows.WaitForSingleObject(h, 0)
	return err == nil && ev == uint32(windows.WAIT_TIMEOUT)
}

// soltarEscondido solta o programa e não deixa abrir janela nenhuma.
func soltarEscondido(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS | windows.CREATE_NO_WINDOW}
}
