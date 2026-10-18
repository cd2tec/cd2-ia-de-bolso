//go:build release

package chave_test

import (
	"encoding/base64"
	"testing"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/chave"
)

// Guarda de release: roda só com -tags release (o job do ZIP do pen drive). Falha enquanto
// Publica for a chave provisória do vetor de teste, cuja privada está no repositório.
func TestPublicaNaoEhADoVetor(t *testing.T) {
	v, _ := carregar(t)
	if len(chave.Publica) != 32 {
		t.Fatalf("chave pública com %d bytes, esperado 32", len(chave.Publica))
	}
	if base64.StdEncoding.EncodeToString(chave.Publica) == v.PublicaBase64 {
		t.Fatal("chave.Publica ainda é a PROVISÓRIA do vetor de teste: cole a chave de produção em instalacao/chave/publica.go antes de gerar o ZIP")
	}
}
