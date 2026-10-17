//go:build !windows

package navegador

import "os/exec"

func ajustar(*exec.Cmd) {}
