package navegador

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// AreaUtil devolve "largura,altura" da tela sem a barra de menu e o Dock, sem
// pedir permissão. Erro, timeout (5 s) ou saída vazia viram "" (sem tamanho explícito).
func AreaUtil() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "osascript", "-l", "JavaScript", "-e",
		`ObjC.import("AppKit"); var f = $.NSScreen.mainScreen.visibleFrame; Math.floor(f.size.width) + "," + Math.floor(f.size.height)`).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
