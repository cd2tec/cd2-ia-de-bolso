//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// iniciarSolto roda o programa num grupo próprio, sem esperar por ele.
func iniciarSolto(exe string, args ...string) error {
	c := exec.Command(exe, args...)
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := c.Start(); err != nil {
		return err
	}
	go func() { _ = c.Wait() }() // só colhe o processo
	return nil
}

// processoVivo diz se o pid ainda existe (EPERM: existe, só não é nosso).
func processoVivo(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
