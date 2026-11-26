//go:build linux

package discos

import "os/exec"

// Padrao devolve o Sistema real deste computador.
func Padrao() Sistema { return NovoLinux(executorReal) }

func ajustarProcesso(*exec.Cmd) {}
