// Package portas escolhe o trio de portas locais da IA.
package portas

import (
	"fmt"
	"net"
	"strconv"
)

const tentativas = 20

// Achar devolve o primeiro P, de inicio a inicio+19, com P, P+1000 e P+3000 livres.
func Achar(inicio int, livre func(int) bool) (int, error) {
	for p := inicio; p < inicio+tentativas; p++ {
		if livre(p) && livre(p+1000) && livre(p+3000) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("nenhum trio de portas livre entre %d e %d", inicio, inicio+tentativas-1)
}

// Livre diz se dá para escutar em 127.0.0.1:p. Fecha na hora.
func Livre(p int) bool {
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(p)))
	if err != nil {
		return false
	}
	l.Close()
	return true
}
