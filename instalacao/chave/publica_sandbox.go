//go:build sandbox

package chave

import "crypto/ed25519"

// Publica, no build de teste (-tags sandbox), é a chave pública do vetor de
// teste (testdata/vetor.json). O Worker de teste assina com a privada desse
// vetor, que está no repositório de propósito. Uma chave emitida pela sandbox
// não abre a IA do build oficial, e uma chave de produção não abre o de teste.
var Publica = ed25519.PublicKey(mustBase64("jllD0Fs5qlMeW6Fq4d1GRAqrQStFjnIsv/SC6Dk7AIE="))
