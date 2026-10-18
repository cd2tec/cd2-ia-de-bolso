//go:build !darwin && !windows

package codigo

import "errors"

// Ler não tem como identificar o computador fora do Windows e do Mac.
func Ler() (string, error) {
	return "", errors.New("este sistema não é suportado para a instalação no computador")
}
