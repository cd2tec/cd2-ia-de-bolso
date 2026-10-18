// Package chave formata, normaliza e confere a chave de instalação: uma
// assinatura Ed25519 do código do computador, escrita em base32 Crockford.
package chave

import (
	"crypto/ed25519"
	"errors"
	"strings"
)

const (
	prefixo   = "CD2"
	alfabeto  = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	tamCorpo  = 103 // 64 bytes = 512 bits, 5 bits por caractere, arredondado para cima
	sigTamanh = ed25519.SignatureSize
)

// Mensagem é o que o servidor assina: cd2-instalacao-v1|<código>.
func Mensagem(codigo string) []byte {
	return []byte("cd2-instalacao-v1|" + codigo)
}

// Formatar escreve a assinatura como CD2- mais base32 Crockford maiúsculo,
// sem preenchimento (107 caracteres no total para 64 bytes).
func Formatar(sig []byte) string {
	var sb strings.Builder
	sb.WriteString(prefixo + "-")
	var acc uint32
	var n uint
	for _, b := range sig {
		acc = acc<<8 | uint32(b)
		n += 8
		for n >= 5 {
			sb.WriteByte(alfabeto[(acc>>(n-5))&31])
			n -= 5
		}
		acc &= 1<<n - 1
	}
	if n > 0 {
		sb.WriteByte(alfabeto[(acc<<(5-n))&31])
	}
	return sb.String()
}

// Normalizar limpa o que a pessoa colou: tira espaços, quebras de linha e
// hífens, passa para maiúsculas, troca O por 0 e I ou L por 1. Exige o prefixo
// CD2 e 103 caracteres do alfabeto, e devolve no formato de Formatar.
func Normalizar(s string) (string, bool) {
	var sb strings.Builder
	for _, r := range strings.ToUpper(s) {
		switch r {
		case ' ', '\t', '\r', '\n', '-':
			continue
		case 'O':
			r = '0'
		case 'I', 'L':
			r = '1'
		}
		sb.WriteRune(r)
	}
	limpo := sb.String()
	if !strings.HasPrefix(limpo, prefixo) {
		return "", false
	}
	corpo := limpo[len(prefixo):]
	if len(corpo) != tamCorpo {
		return "", false
	}
	for i := 0; i < len(corpo); i++ {
		if strings.IndexByte(alfabeto, corpo[i]) < 0 {
			return "", false
		}
	}
	return prefixo + "-" + corpo, true
}

// Decodificar normaliza a chave e devolve os 64 bytes da assinatura. Rejeita
// tamanho errado e bits de preenchimento diferentes de zero.
func Decodificar(chave string) ([]byte, error) {
	norm, ok := Normalizar(chave)
	if !ok {
		return nil, errors.New("chave com formato inválido")
	}
	corpo := norm[len(prefixo)+1:]
	out := make([]byte, 0, sigTamanh)
	var acc uint32
	var n uint
	for i := 0; i < len(corpo); i++ {
		acc = acc<<5 | uint32(strings.IndexByte(alfabeto, corpo[i]))
		n += 5
		if n >= 8 {
			out = append(out, byte(acc>>(n-8)))
			n -= 8
			acc &= 1<<n - 1
		}
	}
	if acc != 0 {
		return nil, errors.New("chave com bits de preenchimento inválidos")
	}
	if len(out) != sigTamanh {
		return nil, errors.New("chave com tamanho inválido")
	}
	return out, nil
}

// ConferirCom diz se a chave é a assinatura de pub para o código do computador.
func ConferirCom(pub ed25519.PublicKey, chave, codigo string) bool {
	if len(pub) != ed25519.PublicKeySize {
		return false
	}
	sig, err := Decodificar(chave)
	if err != nil {
		return false
	}
	return ed25519.Verify(pub, Mensagem(codigo), sig)
}

// Conferir confere a chave com a chave pública embutida (Publica).
func Conferir(chave, codigo string) bool {
	return ConferirCom(Publica, chave, codigo)
}
