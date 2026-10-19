// Package elevacao cuida do instalador que abriu como administrador no
// Windows: diz se está elevado e reabre o programa como o usuário logado,
// para a instalação no computador cair na conta certa.
package elevacao

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// ArgInstalar é o argumento com que o instalador reabre direto no caminho de
// instalar no computador.
const ArgInstalar = "--instalar"

// ArgSinal leva o nonce com que a instância reaberta avisa que abriu.
const ArgSinal = "--sinal="

// tamSinal é o tamanho do nonce em hexadecimal minúsculo.
const tamSinal = 32

// ErrRecusou é a resposta de AbrirComoAdmin quando a pessoa não dá a
// permissão de administrador.
var ErrRecusou = errors.New("a permissão de administrador não foi dada")

// PrazoSinal é quanto a instância elevada espera o sinal da reaberta.
const PrazoSinal = 10 * time.Second

// LerArgs diz se veio --instalar entre os argumentos e devolve o nonce de
// --sinal=, ou "" se não veio ou não tem o formato de NovoSinal. O resto é
// ignorado.
func LerArgs(args []string) (instalar bool, sinal string) {
	for _, a := range args {
		switch {
		case a == ArgInstalar:
			instalar = true
		case strings.HasPrefix(a, ArgSinal) && sinalValido(a[len(ArgSinal):]):
			sinal = a[len(ArgSinal):]
		}
	}
	return instalar, sinal
}

func sinalValido(n string) bool {
	if len(n) != tamSinal {
		return false
	}
	for i := 0; i < len(n); i++ {
		if !strings.ContainsRune("0123456789abcdef", rune(n[i])) {
			return false
		}
	}
	return true
}

// NovoSinal gera o nonce que a instância elevada passa à reaberta.
func NovoSinal() (string, error) {
	b := make([]byte, tamSinal/2)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// nomeSinal é o mutex nomeado que a instância reaberta cria.
func nomeSinal(nonce string) string { return `Local\CD2IADeBolso-` + nonce }

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
	sb.WriteString(`/d /v:off /c set __COMPAT_LAYER=RunAsInvoker&& start ""`)
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
