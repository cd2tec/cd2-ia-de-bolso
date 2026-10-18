//go:build !darwin && !windows

package origem

import "errors"

func lerVolumeDoSistema(string) (Volume, error) {
	return Volume{}, errors.New("este sistema não é suportado")
}
