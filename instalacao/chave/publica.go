package chave

import (
	"crypto/ed25519"
	"encoding/base64"
)

// Publica é a chave pública que confere as chaves de instalação.
//
// PROVISÓRIA: esta é a chave pública do vetor de teste (testdata/vetor.json),
// cuja chave privada está no repositório de propósito. A chave de produção
// substitui esta antes de qualquer release (a Task 13 adiciona a conferência
// no release). Nunca publicar uma versão com esta chave.
var Publica = ed25519.PublicKey(mustBase64("jllD0Fs5qlMeW6Fq4d1GRAqrQStFjnIsv/SC6Dk7AIE="))

func mustBase64(s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
