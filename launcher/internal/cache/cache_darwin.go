package cache

import "os/exec"

// tirarQuarentena tira o com.apple.quarantine para o Gatekeeper não barrar o
// motor copiado. Erro é ignorado: o atributo costuma nem existir.
func tirarQuarentena(destino string) {
	_ = exec.Command("xattr", "-d", "com.apple.quarantine", destino).Run()
}
