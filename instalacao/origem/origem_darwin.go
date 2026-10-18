//go:build darwin

package origem

import (
	"fmt"
	"path/filepath"
	"syscall"
)

// pontoDeMontagem sobe da raiz até achar o ponto de montagem do volume.
func pontoDeMontagem(raiz string) (string, error) {
	abs, err := filepath.Abs(raiz)
	if err != nil {
		return "", err
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(abs, &st); err != nil {
		return "", fmt.Errorf("não consegui ler o volume de %s: %w", abs, err)
	}
	var ponto []byte
	for _, c := range st.Mntonname {
		if c == 0 {
			break
		}
		ponto = append(ponto, byte(c))
	}
	if len(ponto) == 0 {
		return "", fmt.Errorf("volume de %s sem ponto de montagem", abs)
	}
	return string(ponto), nil
}

func lerVolumeDoSistema(raiz string) (Volume, error) {
	ponto, err := pontoDeMontagem(raiz)
	if err != nil {
		return Volume{}, err
	}
	if montagemDoSistema(ponto) {
		return Volume{DoSistema: true}, nil
	}
	return volumeDoDiskutil(ponto)
}
