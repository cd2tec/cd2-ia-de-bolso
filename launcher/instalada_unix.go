//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// soltar põe o programa num grupo próprio: ele sobrevive à saída deste.
func soltar(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// processoVivo diz se o pid ainda existe (EPERM: existe, só não é nosso).
func processoVivo(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
