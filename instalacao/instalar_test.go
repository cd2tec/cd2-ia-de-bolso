package instalacao

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/copiar"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/guarda"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/sistema"
)

// sisFalso anota o que o orquestrador pediu ao sistema.
type sisFalso struct {
	chamadas   []string
	inicio     bool
	erroAtalho error
}

func (s *sisFalso) Atalhos(l string, area bool) error {
	s.chamadas = append(s.chamadas, "Atalhos "+l+" "+sim(area))
	return s.erroAtalho
}

func (s *sisFalso) LigarInicio(l string, ligar bool) error {
	s.chamadas = append(s.chamadas, "LigarInicio "+l+" "+sim(ligar))
	s.inicio = ligar
	return nil
}

func (s *sisFalso) InicioLigado() bool { return s.inicio }

func (s *sisFalso) Registrar(l, v string) error {
	s.chamadas = append(s.chamadas, "Registrar "+l+" "+v)
	return nil
}

func (s *sisFalso) Remover() error {
	s.chamadas = append(s.chamadas, "Remover")
	return nil
}

func (s *sisFalso) PlanoRemocao(destino string, pid int) sistema.Comando {
	return sistema.Comando{Exe: "apagar", Args: []string{destino}}
}

func sim(b bool) string {
	if b {
		return "sim"
	}
	return "não"
}

// trocar redireciona sistema, destino, chave e cópia para o teste.
func trocar(t *testing.T, chaveBoa bool) (*sisFalso, string, *[]string) {
	t.Helper()
	sis := &sisFalso{}
	destino := filepath.Join(t.TempDir(), "IA de Bolso")
	var copiados []string
	velhos := []any{sistemaDo, pastaDestino, valida, planejar, copiarPara, goos}
	sistemaDo = func(string) sistema.Sistema { return sis }
	pastaDestino = func() (string, error) { return destino, nil }
	valida = func(string) bool { return chaveBoa }
	goos = "darwin"
	planejar = func(raiz, g, a string) ([]copiar.Item, int64, error) {
		return []copiar.Item{{Origem: raiz + "/VERSION", Destino: "VERSION", Tamanho: 1}}, 1, nil
	}
	copiarPara = func(ctx context.Context, itens []copiar.Item, d string, prog func(int64, int64)) error {
		copiados = append(copiados, d)
		return os.MkdirAll(d, 0o755)
	}
	t.Cleanup(func() {
		sistemaDo = velhos[0].(func(string) sistema.Sistema)
		pastaDestino = velhos[1].(func() (string, error))
		valida = velhos[2].(func(string) bool)
		planejar = velhos[3].(func(string, string, string) ([]copiar.Item, int64, error))
		copiarPara = velhos[4].(func(context.Context, []copiar.Item, string, func(int64, int64)) error)
		goos = velhos[5].(string)
	})
	return sis, destino, &copiados
}

func TestInstalarSemChaveRecusa(t *testing.T) {
	sis, destino, copiados := trocar(t, false)
	err := Instalar(context.Background(), Opcoes{Raiz: t.TempDir(), Cache: t.TempDir(), Versao: "1.0.0"}, nil)
	if !errors.Is(err, ErrSemChave) {
		t.Fatalf("err = %v, quero ErrSemChave", err)
	}
	if err.Error() != "Falta a chave deste computador." {
		t.Errorf("mensagem = %q", err.Error())
	}
	if len(*copiados) != 0 || len(sis.chamadas) != 0 {
		t.Errorf("mexeu em algo sem chave: copiou %v, chamou %v", *copiados, sis.chamadas)
	}
	if _, err := os.Stat(destino); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("destino criado sem chave: %v", err)
	}
}

func TestInstalarSemChaveDeVerdadeRecusa(t *testing.T) {
	// Sem trocar o valida: cache vazio não tem chave nenhuma.
	_, _, copiados := trocar(t, false)
	valida = guarda.Valida
	err := Instalar(context.Background(), Opcoes{Raiz: t.TempDir(), Cache: t.TempDir()}, nil)
	if !errors.Is(err, ErrSemChave) || len(*copiados) != 0 {
		t.Fatalf("err = %v, copiados = %v", err, *copiados)
	}
}

func TestInstalarNaOrdem(t *testing.T) {
	sis, destino, copiados := trocar(t, true)
	o := Opcoes{Raiz: t.TempDir(), Cache: t.TempDir(), Versao: "1.4.0", AreaDeTrabalho: true, Inicio: false}
	if err := Instalar(context.Background(), o, nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*copiados, []string{destino}) {
		t.Errorf("copiou para %v, quero %s", *copiados, destino)
	}
	lanc := sistema.Lancador(destino, "darwin")
	quer := []string{"Registrar " + lanc + " 1.4.0", "Atalhos " + lanc + " sim", "LigarInicio " + lanc + " não"}
	if !reflect.DeepEqual(sis.chamadas, quer) {
		t.Errorf("chamadas = %q\nquero      %q", sis.chamadas, quer)
	}
}

func TestInstalarErroNaCopiaParaAntesDosAtalhos(t *testing.T) {
	sis, _, _ := trocar(t, true)
	copiarPara = func(context.Context, []copiar.Item, string, func(int64, int64)) error { return copiar.ErrSemEspaco }
	err := Instalar(context.Background(), Opcoes{Raiz: t.TempDir(), Cache: t.TempDir()}, nil)
	if !errors.Is(err, copiar.ErrSemEspaco) {
		t.Fatalf("err = %v", err)
	}
	if len(sis.chamadas) != 0 {
		t.Errorf("mexeu no sistema depois de a cópia falhar: %v", sis.chamadas)
	}
}

func TestInstalarAtalhoFalhoViraAviso(t *testing.T) {
	sis, destino, _ := trocar(t, true)
	sis.erroAtalho = errors.New("sem permissão")
	err := Instalar(context.Background(), Opcoes{Raiz: t.TempDir(), Cache: t.TempDir(), Versao: "1.0.0", Inicio: true}, nil)
	if !errors.Is(err, ErrAvisoAtalhos) {
		t.Fatalf("err = %v, quero ErrAvisoAtalhos", err)
	}
	if !contem(sis.chamadas, "Registrar "+sistema.Lancador(destino, "darwin")+" 1.0.0") {
		t.Errorf("não registrou: %v", sis.chamadas)
	}
	if !sis.inicio {
		t.Error("parou antes de ligar o início")
	}
	if _, err := os.Stat(destino); err != nil {
		t.Errorf("arquivos não ficaram: %v", err)
	}
}

func TestInstalarSemPastaPessoalRecusa(t *testing.T) {
	_, _, copiados := trocar(t, true)
	pastaDestino = func() (string, error) { return "", sistema.ErrSemPastaPessoal }
	err := Instalar(context.Background(), Opcoes{Raiz: t.TempDir(), Cache: t.TempDir()}, nil)
	if !errors.Is(err, sistema.ErrSemPastaPessoal) || len(*copiados) != 0 {
		t.Fatalf("err = %v, copiados = %v", err, *copiados)
	}
}

func TestDestinoDoSistemaSemHome(t *testing.T) {
	if _, err := destinoDoSistema("darwin", func(string) string { return "" }, ""); err == nil {
		t.Error("sem home no Mac devia dar erro")
	}
	if _, err := destinoDoSistema("windows", func(string) string { return "" }, ""); err == nil {
		t.Error("sem home nem LOCALAPPDATA no Windows devia dar erro")
	}
	d, err := destinoDoSistema("darwin", nil, "/Users/ana")
	if err != nil || d != "/Users/ana/Applications/IA de Bolso" {
		t.Errorf("d = %q, err = %v", d, err)
	}
}

func TestDesinstalarRecusaPastaEstranha(t *testing.T) {
	sis, _, _ := trocar(t, true)
	pastaDestino = func() (string, error) { return t.TempDir(), nil }
	if _, err := Desinstalar(t.TempDir(), 10); err == nil {
		t.Error("devia recusar pasta que não é a da instalação")
	}
	if len(sis.chamadas) != 0 {
		t.Errorf("mexeu no sistema: %v", sis.chamadas)
	}
}

func TestDesinstalarGuardaAChaveEApagaDbEnv(t *testing.T) {
	sis, destino, _ := trocar(t, true)
	sis.inicio = true
	cache := t.TempDir()
	for _, n := range []string{"instalacao.chave", "db.env"} {
		if err := os.WriteFile(filepath.Join(cache, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	c, err := Desinstalar(cache, 4242)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cache, "instalacao.chave")); err != nil {
		t.Errorf("a chave sumiu: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cache, "db.env")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("db.env ficou: %v", err)
	}
	if sis.inicio {
		t.Error("o início continuou ligado")
	}
	if !contem(sis.chamadas, "Remover") {
		t.Errorf("não removeu atalhos e registro: %v", sis.chamadas)
	}
	if c.Exe != "apagar" || len(c.Args) != 1 || c.Args[0] != destino {
		t.Errorf("comando = %#v, quero o plano de remoção de %s", c, destino)
	}

	// De novo, já sem db.env: não é erro.
	if _, err := Desinstalar(cache, 4242); err != nil {
		t.Errorf("segunda desinstalação: %v", err)
	}
}

func contem(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
