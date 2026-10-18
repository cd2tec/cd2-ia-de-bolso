//go:build windows

package copiar

import "golang.org/x/sys/windows"

// EspacoLivre devolve os bytes livres para o usuário no disco de dir.
func EspacoLivre(dir string) (int64, error) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var livre, total, totalLivre uint64
	if err := windows.GetDiskFreeSpaceEx(p, &livre, &total, &totalLivre); err != nil {
		return 0, err
	}
	return int64(livre), nil
}
