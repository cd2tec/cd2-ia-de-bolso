//go:build darwin

package discos

import "os/exec"

// Padrao devolve o Sistema real deste computador.
func Padrao() Sistema { return NovoMac(executorReal) }

func ajustarProcesso(*exec.Cmd) {}
