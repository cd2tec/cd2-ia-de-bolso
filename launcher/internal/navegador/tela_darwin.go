package navegador

import (
	"os/exec"
	"strings"
)

// AreaUtil devolve "largura,altura" da tela sem a barra de menu e o Dock, sem
// pedir permissão. Erro ou saída vazia viram "" (sem tamanho explícito).
func AreaUtil() string {
	out, err := exec.Command("osascript", "-l", "JavaScript", "-e",
		`ObjC.import("AppKit"); var f = $.NSScreen.mainScreen.visibleFrame; Math.floor(f.size.width) + "," + Math.floor(f.size.height)`).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
