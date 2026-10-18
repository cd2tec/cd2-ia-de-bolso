package main

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// iniciarSolto roda o programa solto deste processo, sem esperar por ele.
func iniciarSolto(exe string, args ...string) error {
	c := exec.Command(exe, args...)
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS}
	if err := c.Start(); err != nil {
		return err
	}
	go func() { _ = c.Wait() }()
	return nil
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
