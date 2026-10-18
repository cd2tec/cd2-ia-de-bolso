package chave_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/chave"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/codigo"
)

type vetor struct {
	PrivadaPKCS8Base64 string `json:"privadaPKCS8Base64"`
	PublicaBase64      string `json:"publicaBase64"`
	Identidade         string `json:"identidade"`
	Codigo             string `json:"codigo"`
	Chave              string `json:"chave"`
}

func carregar(t *testing.T) (vetor, ed25519.PublicKey) {
	t.Helper()
	b, err := os.ReadFile("../testdata/vetor.json")
	if err != nil {
		t.Fatal(err)
	}
	var v vetor
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	pub, err := base64.StdEncoding.DecodeString(v.PublicaBase64)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		t.Fatal("publica do vetor inválida", err)
	}
	return v, ed25519.PublicKey(pub)
}

func TestVetorCompartilhado(t *testing.T) {
	v, pub := carregar(t)
	if codigo.Derivar(v.Identidade) != v.Codigo {
		t.Fatal("código")
	}
	if !chave.ConferirCom(pub, v.Chave, v.Codigo) {
		t.Fatal("chave do vetor")
	}
	if chave.ConferirCom(pub, v.Chave, "AAAA-AAAA-AAAA") {
		t.Fatal("aceitou outro computador")
	}
}

func TestConferirUsaChavePublica(t *testing.T) {
	v, pub := carregar(t)
	// Com a chave pública de produção colada em publica.go, o vetor de teste deixa de valer
	// para Conferir; o caso continua coberto por ConferirCom (TestVetorCompartilhado).
	if !bytes.Equal(chave.Publica, pub) {
		t.Skip("Publica já não é a do vetor de teste (chave de produção)")
	}
	if !chave.Conferir(v.Chave, v.Codigo) {
		t.Fatal("Publica é a do vetor, então a chave do vetor deveria conferir")
	}
}

func TestFormatarTamanho(t *testing.T) {
	if n := len(chave.Formatar(make([]byte, 64))); n != 107 {
		t.Fatal(n)
	}
}

func TestMensagem(t *testing.T) {
	if string(chave.Mensagem("AAAA-BBBB-CCCC")) != "cd2-instalacao-v1|AAAA-BBBB-CCCC" {
		t.Fatal("mensagem")
	}
}

func TestNormalizarAceitaColagemSuja(t *testing.T) {
	v, _ := carregar(t)
	sujo := " cd2-" + strings.ToLower(v.Chave[4:20]) + "\n-" + v.Chave[20:] + "  "
	got, ok := chave.Normalizar(sujo)
	if !ok || got != v.Chave {
		t.Fatal(got)
	}
	if _, ok := chave.Normalizar("CD2-curta"); ok {
		t.Fatal("aceitou chave curta")
	}
}

func TestNormalizarTrocaConfundiveis(t *testing.T) {
	// Chave só de zeros e uns, escrita com O, I e L no lugar.
	base := "CD2-" + strings.Repeat("0", 50) + strings.Repeat("1", 53)
	sujo := "CD2-" + strings.Repeat("O", 50) + strings.Repeat("I", 20) + strings.Repeat("l", 33)
	got, ok := chave.Normalizar(sujo)
	if !ok || got != base {
		t.Fatal(got, ok)
	}
}

func TestNormalizarRejeitaInvalidos(t *testing.T) {
	corpo := strings.Repeat("0", 103)
	casos := map[string]string{
		"sem prefixo":       corpo,
		"prefixo errado":    "XYZ-" + corpo,
		"letra U":           "CD2-" + strings.Repeat("0", 102) + "U",
		"longa demais":      "CD2-" + corpo + "0",
		"caractere de lixo": "CD2-" + strings.Repeat("0", 102) + "!",
	}
	for nome, s := range casos {
		if _, ok := chave.Normalizar(s); ok {
			t.Errorf("%s: aceitou", nome)
		}
	}
}

func TestBitsDePreenchimentoNaoZeroRejeitados(t *testing.T) {
	// 64 bytes = 512 bits = 102 chars + 2 bits; o último caractere carrega 2 bits úteis e 3 de preenchimento.
	ok := chave.Formatar(make([]byte, 64))
	ruim := ok[:len(ok)-1] + "1" // 00001: bits de preenchimento não zero
	if _, v := chave.Normalizar(ruim); v {
		// Normalizar só valida o alfabeto; a decodificação é quem rejeita.
		if chave.ConferirCom(make([]byte, ed25519.PublicKeySize), ruim, "AAAA-AAAA-AAAA") {
			t.Fatal("aceitou preenchimento não zero")
		}
	}
	if _, err := chave.Decodificar(ruim); err == nil {
		t.Fatal("Decodificar aceitou preenchimento não zero")
	}
	if _, err := chave.Decodificar(ok); err != nil {
		t.Fatal(err)
	}
	if _, err := chave.Decodificar(ok[:len(ok)-1]); err == nil {
		t.Fatal("Decodificar aceitou tamanho errado")
	}
}

func TestIdaEVolta(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	sig := ed25519.Sign(priv, chave.Mensagem("ABCD-EFGH-JKMN"))
	k := chave.Formatar(sig)
	if !chave.ConferirCom(priv.Public().(ed25519.PublicKey), k, "ABCD-EFGH-JKMN") {
		t.Fatal("ida e volta")
	}
	if chave.ConferirCom(priv.Public().(ed25519.PublicKey), k, "ABCD-EFGH-JKMP") {
		t.Fatal("outro código")
	}
}
