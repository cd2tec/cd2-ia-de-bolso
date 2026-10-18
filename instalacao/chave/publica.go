package chave

import (
	"crypto/ed25519"
	"encoding/base64"
)

// Publica é a chave pública de produção que confere as chaves de instalação.
//
// Gerada em 18/10/2026. A privada correspondente fica só no Worker
// (segredo CHAVE_PRIVADA) e numa cópia guardada pelo Gabriel; nunca no
// repositório. Trocar este par invalida todas as chaves já vendidas.
var Publica = ed25519.PublicKey(mustBase64("hLb6GtKWgRHHhl25qeMBoujsFrngZBlJ+6tjm0ZbU1I="))

func mustBase64(s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
