// Package codigo calcula o código do computador: um identificador curto e
// estável, derivado da identidade que o sistema operacional guarda da máquina.
package codigo

import (
	"crypto/sha256"
	"strings"
)

// alfabeto é o base32 Crockford (sem I, L, O e U).
const alfabeto = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// Derivar transforma a identidade da máquina em XXXX-XXXX-XXXX: SHA-256 de
// um prefixo fixo mais a identidade em minúsculas e sem espaços nas pontas,
// com os 60 primeiros bits em base32 Crockford.
func Derivar(identidade string) string {
	id := strings.ToLower(strings.TrimSpace(identidade))
	soma := sha256.Sum256([]byte("cd2-iadebolso-codigo-v1|" + id))
	var bits uint64
	for _, b := range soma[:8] {
		bits = bits<<8 | uint64(b)
	}
	bits >>= 4 // 64 bits lidos, ficam os 60 primeiros
	var sb strings.Builder
	for i := 0; i < 12; i++ {
		if i > 0 && i%4 == 0 {
			sb.WriteByte('-')
		}
		sb.WriteByte(alfabeto[(bits>>(55-5*uint(i)))&31])
	}
	return sb.String()
}
