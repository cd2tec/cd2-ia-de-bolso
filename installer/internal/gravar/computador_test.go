package gravar

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/preparo"
)

// trocarInstalar troca o instalarFn pelo falso durante o teste.
func trocarInstalar(t *testing.T, f func(context.Context, instalacao.Opcoes, func(int64, int64)) error) {
	t.Helper()
	antigo := instalarFn
	instalarFn = f
	t.Cleanup(func() { instalarFn = antigo })
}

func (a *ambiente) opcoesComputador() OpcoesComputador {
	return OpcoesComputador{
		Cache: a.cache, CacheChave: filepath.Join(a.t.TempDir(), "chave"),
		URLManifesto: a.url, HTTP: a.srv.Client(),
		AreaDeTrabalho: true, Inicio: false,
	}
}

func TestInstalarNoComputadorMontaEInstala(t *testing.T) {
	a := novoAmbiente(t)
	o := a.opcoesComputador()
	var recebeu instalacao.Opcoes
	trocarInstalar(t, func(_ context.Context, io instalacao.Opcoes, prog func(int64, int64)) error {
		recebeu = io
		for _, f := range []int64{0, 50, 100} {
			prog(f, 100)
		}
		return nil
	})
	var eventos []Evento
	if err := InstalarNoComputador(context.Background(), o, func(e Evento) { eventos = append(eventos, e) }); err != nil {
		t.Fatal(err)
	}
	if recebeu.Raiz != filepath.Join(a.cache, "CD2-HARNESS") {
		t.Errorf("Raiz = %q", recebeu.Raiz)
	}
	if recebeu.Cache != o.CacheChave {
		t.Errorf("Cache = %q, quero %q", recebeu.Cache, o.CacheChave)
	}
	if !recebeu.AreaDeTrabalho || recebeu.Inicio {
		t.Errorf("opções = %+v", recebeu)
	}
	if recebeu.Versao != "0.13.0" {
		t.Errorf("Versao = %q", recebeu.Versao)
	}

	var etapas []string
	ant := -1.0
	for _, e := range eventos {
		if e.Fracao < ant {
			t.Fatalf("fração diminuiu de %v para %v em %+v", ant, e.Fracao, e)
		}
		ant = e.Fracao
		if len(etapas) == 0 || etapas[len(etapas)-1] != e.Etapa {
			etapas = append(etapas, e.Etapa)
		}
	}
	if !strings.HasSuffix(strings.Join(etapas, ","), "instalando,pronto") || etapas[0] != "baixando" {
		t.Fatalf("etapas = %v", etapas)
	}
	if !strings.Contains(strings.Join(etapas, ","), "conferindo") {
		t.Errorf("sem etapa conferindo: %v", etapas)
	}
	u := eventos[len(eventos)-1]
	if u.Etapa != "pronto" || u.Fracao != 1 {
		t.Fatalf("último = %+v", u)
	}
	for _, e := range eventos[:len(eventos)-1] {
		if e.Fracao >= 1 {
			t.Fatalf("fração 1 antes do pronto: %+v", e)
		}
	}
}

func TestInstalarNoComputadorAvisoNaoEhErro(t *testing.T) {
	a := novoAmbiente(t)
	trocarInstalar(t, func(context.Context, instalacao.Opcoes, func(int64, int64)) error {
		return errors.Join(instalacao.ErrAvisoAtalhos, errors.New("atalho falhou"))
	})
	var ultimo Evento
	if err := InstalarNoComputador(context.Background(), a.opcoesComputador(), func(e Evento) { ultimo = e }); err != nil {
		t.Fatalf("aviso virou erro: %v", err)
	}
	if ultimo.Etapa != "pronto" || ultimo.Fracao != 1 || ultimo.Frase != instalacao.ErrAvisoAtalhos.Error() {
		t.Fatalf("último = %+v", ultimo)
	}
}

func TestInstalarNoComputadorSemChave(t *testing.T) {
	a := novoAmbiente(t)
	trocarInstalar(t, func(context.Context, instalacao.Opcoes, func(int64, int64)) error {
		return instalacao.ErrSemChave
	})
	err := InstalarNoComputador(context.Background(), a.opcoesComputador(), nil)
	if got := fraseDe(t, err); got != "Falta a chave deste computador." {
		t.Fatalf("frase = %q", got)
	}
	if !errors.Is(err, instalacao.ErrSemChave) {
		t.Fatalf("perdeu a causa: %v", err)
	}
}

func TestInstalarNoComputadorDownloadFalhaNaoInstala(t *testing.T) {
	a := novoAmbiente(t)
	a.falhas["/modelo"] = true
	chamou := false
	trocarInstalar(t, func(context.Context, instalacao.Opcoes, func(int64, int64)) error {
		chamou = true
		return nil
	})
	err := InstalarNoComputador(context.Background(), a.opcoesComputador(), nil)
	if got := fraseDe(t, err); got != fraseInternet {
		t.Fatalf("frase = %q", got)
	}
	if chamou {
		t.Fatal("instalou com o download quebrado")
	}
}

func TestEspacoParaInstalar(t *testing.T) {
	a := novoAmbiente(t)
	antigo := espacoLivreFn
	var pediu string
	espacoLivreFn = func(dir string) (int64, error) { pediu = dir; return 42, nil }
	t.Cleanup(func() { espacoLivreFn = antigo })

	o := a.opcoesComputador()
	precisa, livre, err := EspacoParaInstalar(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	falta := preparo.BytesParaBaixar(a.m, a.cache)
	if quero := int64(float64(falta+3_000_000_000) * 1.1); precisa != quero {
		t.Fatalf("precisa = %d, quero %d", precisa, quero)
	}
	if livre != 42 || pediu == "" {
		t.Fatalf("livre = %d, pediu %q", livre, pediu)
	}
}
