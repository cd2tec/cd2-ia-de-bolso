package portas

import (
	"net"
	"strconv"
	"testing"
)

func TestAcharPulaTrioOcupado(t *testing.T) {
	ocupadas := map[int]bool{8080: true, 9081: true}
	p, err := Achar(8080, func(p int) bool { return !ocupadas[p] })
	if err != nil || p != 8082 {
		t.Fatalf("p = %d, err = %v (esperava 8082)", p, err)
	}
}

func TestAcharDesisteDepoisDe20(t *testing.T) {
	if _, err := Achar(8080, func(int) bool { return false }); err == nil {
		t.Fatal("esperava erro")
	}
}

func TestAcharTentaSoVinte(t *testing.T) {
	// 8100 estaria livre, mas fica fora da janela de 20 tentativas (8080 a 8099).
	if _, err := Achar(8080, func(p int) bool { return p == 8100 }); err == nil {
		t.Fatal("esperava erro")
	}
}

func TestLivre(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := l.Addr().(*net.TCPAddr).Port
	if Livre(p) {
		t.Fatalf("porta %s ocupada aparece como livre", strconv.Itoa(p))
	}
	l.Close()
	if !Livre(p) {
		t.Fatal("porta liberada aparece como ocupada")
	}
}
