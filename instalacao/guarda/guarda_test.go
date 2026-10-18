package guarda

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/chave"
)

func vetor(t *testing.T) (pub ed25519.PublicKey, codigo, chv string) {
	t.Helper()
	b, err := os.ReadFile("../testdata/vetor.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		PublicaBase64 string `json:"publicaBase64"`
		Codigo        string `json:"codigo"`
		Chave         string `json:"chave"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	p, err := base64.StdEncoding.DecodeString(v.PublicaBase64)
	if err != nil {
		t.Fatal(err)
	}
	return ed25519.PublicKey(p), v.Codigo, v.Chave
}

func TestGuardaIdaEVolta(t *testing.T) {
	_, _, chv := vetor(t)
	pasta := filepath.Join(t.TempDir(), "cache", "cd2")
	suja := "  " + chv[:20] + " \n" + chv[20:] + "\n"
	if err := Gravar(pasta, suja); err != nil {
		t.Fatal(err)
	}
	quer, ok := chave.Normalizar(chv)
	if !ok {
		t.Fatal("vetor não normaliza")
	}
	if got := Ler(pasta); got != quer {
		t.Fatalf("Ler = %q, quer %q", got, quer)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(pasta, "instalacao.chave"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("permissão do arquivo = %v", info.Mode().Perm())
		}
		d, _ := os.Stat(pasta)
		if d.Mode().Perm() != 0o700 {
			t.Fatalf("permissão da pasta = %v", d.Mode().Perm())
		}
	}
}

func TestGravarRejeitaChaveInvalida(t *testing.T) {
	pasta := t.TempDir()
	if err := Gravar(pasta, "isto não é chave"); err == nil {
		t.Fatal("aceitou chave inválida")
	}
	if Ler(pasta) != "" {
		t.Fatal("gravou algo")
	}
}

func TestGuardaVaziaQuandoNaoExiste(t *testing.T) {
	if got := Ler(t.TempDir()); got != "" {
		t.Fatalf("Ler = %q", got)
	}
}

func TestValida(t *testing.T) {
	pub, cod, chv := vetor(t)
	antesL, antesC := lerCodigo, conferir
	defer func() { lerCodigo, conferir = antesL, antesC }()
	conferir = func(c, k string) bool { return chave.ConferirCom(pub, c, k) }
	pasta := t.TempDir()

	lerCodigo = func() (string, error) { return cod, nil }
	if Valida(pasta) {
		t.Fatal("válida sem chave gravada")
	}
	if err := Gravar(pasta, chv); err != nil {
		t.Fatal(err)
	}
	if !Valida(pasta) {
		t.Fatal("chave do vetor deveria valer")
	}
	lerCodigo = func() (string, error) { return "AAAA-AAAA-AAAA", nil }
	if Valida(pasta) {
		t.Fatal("valeu em outro computador")
	}
	lerCodigo = func() (string, error) { return "", os.ErrNotExist }
	if Valida(pasta) {
		t.Fatal("valeu sem conseguir ler o código")
	}
}

func TestGravarCorrigePermissaoDeArquivoAntigo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permissões POSIX")
	}
	_, _, chv := vetor(t)
	pasta := t.TempDir()
	arq := filepath.Join(pasta, "instalacao.chave")
	if err := os.WriteFile(arq, []byte("velho"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Gravar(pasta, chv); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(arq)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissão = %v", info.Mode().Perm())
	}
}
