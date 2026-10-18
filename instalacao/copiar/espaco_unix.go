//go:build !windows

package copiar

import "golang.org/x/sys/unix"

// EspacoLivre devolve os bytes livres para quem não é root no disco de dir.
func EspacoLivre(dir string) (int64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}
