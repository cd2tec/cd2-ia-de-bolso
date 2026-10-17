//go:build !windows

package sobra

// Fora do Windows não há o que encerrar.
func listar() ([]Processo, error) { return nil, nil }

func matar(uint32) error { return nil }
