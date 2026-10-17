package chave

import (
	"regexp"
	"testing"
)

func TestChaveNova(t *testing.T) {
	a, err := Nova()
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 64 || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(a) {
		t.Fatalf("chave fora do formato: %q", a)
	}
	b, _ := Nova()
	if a == b {
		t.Fatal("duas chaves iguais")
	}
}
