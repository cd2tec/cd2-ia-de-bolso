//go:build !darwin

package cache

// tirarQuarentena não faz nada fora do Mac.
func tirarQuarentena(string) {}
