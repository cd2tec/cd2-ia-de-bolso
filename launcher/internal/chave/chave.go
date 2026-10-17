// Package chave gera a chave da sessão que protege o cd2-ctl.
package chave

import (
	"crypto/rand"
	"encoding/hex"
)

// Nova devolve 32 bytes aleatórios em hexadecimal minúsculo (64 caracteres).
func Nova() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
