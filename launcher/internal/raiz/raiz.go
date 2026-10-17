// Package raiz descobre a raiz do pen drive a partir do caminho do lançador.
package raiz

import (
	"fmt"
	"path"
	"strings"
)

// DoExecutavel devolve a raiz do pen drive. O caminho é tratado conforme o
// goos informado, e não o do sistema em que o código roda, para que os testes
// funcionem em qualquer máquina.
func DoExecutavel(exe string, goos string) (string, error) {
	switch goos {
	case "windows":
		i := strings.LastIndex(exe, `\`)
		if i < 0 {
			return "", fmt.Errorf("caminho sem pasta: %q", exe)
		}
		dir := exe[:i]
		// Raiz de drive ("E:") precisa da barra final para continuar sendo um caminho.
		if len(dir) == 2 && dir[1] == ':' {
			dir += `\`
		}
		return dir, nil
	case "darwin":
		// X/IA de Bolso.app/Contents/MacOS/IA de Bolso  ->  X
		const marca = ".app/Contents/MacOS/"
		i := strings.LastIndex(exe, marca)
		if i < 0 {
			return "", fmt.Errorf("o executável não está dentro de um .app: %q", exe)
		}
		app := exe[:i+len(".app")]
		return path.Dir(app), nil
	}
	return "", fmt.Errorf("sistema sem suporte: %q", goos)
}
