// Package elevacao cuida do instalador que abriu como administrador no
// Windows: diz se está elevado e reabre o programa como o usuário logado,
// para a instalação no computador cair na conta certa.
package elevacao

import (
	"errors"
	"strings"
)

// ArgInstalar é o argumento com que o instalador reabre direto no caminho de
// instalar no computador.
const ArgInstalar = "--instalar"

// LerArgs diz se veio --instalar entre os argumentos. O resto é ignorado.
func LerArgs(args []string) bool {
	for _, a := range args {
		if a == ArgInstalar {
			return true
		}
	}
	return false
}

// parametrosSemAdmin monta a linha do cmd.exe que o Explorer roda para
// reabrir o instalador sem administrador. O manifesto do instalador pede
// administrador; com __COMPAT_LAYER=RunAsInvoker o Windows abre o programa
// com o nível de quem chamou (o Explorer, sem administrador) em vez de pedir
// a elevação de novo. Caminho ou argumento com aspas, % ou quebra de linha é
// recusado, porque o cmd os interpretaria.
func parametrosSemAdmin(exe string, args []string) (string, error) {
	if exe == "" {
		return "", errors.New("sem o caminho do programa")
	}
	var sb strings.Builder
	sb.WriteString(`/d /c set __COMPAT_LAYER=RunAsInvoker&& start ""`)
	for _, p := range append([]string{exe}, args...) {
		if strings.ContainsAny(p, "\"%\r\n") {
			return "", errors.New("caminho ou argumento que o cmd não aceita: " + p)
		}
		sb.WriteString(` "`)
		sb.WriteString(p)
		sb.WriteString(`"`)
	}
	return sb.String(), nil
}
