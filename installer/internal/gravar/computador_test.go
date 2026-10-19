package gravar

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/baixar"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/manifesto"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/preparo"
)

// trocarInstalar troca o instalarFn pelo falso durante o teste.
func trocarInstalar(t *testing.T, f func(context.Context, instalacao.Opcoes, func(int64, int64)) error) {
	t.Helper()
	antigo := instalarFn
	instalarFn = f
	t.Cleanup(func() { instalarFn = antigo })
}

// trocarChave faz a conferência da chave responder valida durante o teste.
func trocarChave(t *testing.T, valida bool) {
	t.Helper()
	antigo := validaChaveFn
	validaChaveFn = func(string) bool { return valida }
	t.Cleanup(func() { validaChaveFn = antigo })
}

// opcoesComputador já deixa a chave valendo; o teste que quer sem chave
// chama trocarChave(t, false) depois.
func (a *ambiente) opcoesComputador() OpcoesComputador {
	trocarChave(a.t, true)
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
	o := a.opcoesComputador()
	trocarChave(t, false)
	chamou := false
	trocarInstalar(t, func(context.Context, instalacao.Opcoes, func(int64, int64)) error {
		chamou = true
		return nil
	})
	err := InstalarNoComputador(context.Background(), o, nil)
	if got := fraseDe(t, err); got != "Falta a chave deste computador." {
		t.Fatalf("frase = %q", got)
	}
	if !errors.Is(err, instalacao.ErrSemChave) {
		t.Fatalf("perdeu a causa: %v", err)
	}
	// Nem o manifesto foi pedido: o Montar nunca rodou.
	if n := a.requisicoes(); n != 0 {
		t.Fatalf("baixou sem chave: %d requisições", n)
	}
	if chamou {
		t.Fatal("chamou o Instalar sem chave")
	}
}

// A chave que vence entre a conferência e a cópia ainda vira a frase certa.
func TestInstalarNoComputadorSemChaveNoInstalar(t *testing.T) {
	a := novoAmbiente(t)
	trocarInstalar(t, func(context.Context, instalacao.Opcoes, func(int64, int64)) error {
		return instalacao.ErrSemChave
	})
	err := InstalarNoComputador(context.Background(), a.opcoesComputador(), nil)
	if got := fraseDe(t, err); got != "Falta a chave deste computador." {
		t.Fatalf("frase = %q", got)
	}
}

func TestInstalarNoComputadorBarraChegaPertoDoFim(t *testing.T) {
	a := novoAmbiente(t)
	trocarInstalar(t, func(_ context.Context, _ instalacao.Opcoes, prog func(int64, int64)) error {
		// O total da cópia é bem menor que o stage (só esta plataforma).
		for f := int64(0); f <= 10; f++ {
			prog(f, 10)
		}
		return nil
	})
	var ultimo Evento
	if err := InstalarNoComputador(context.Background(), a.opcoesComputador(), func(e Evento) {
		if e.Etapa == "instalando" {
			ultimo = e
		}
	}); err != nil {
		t.Fatal(err)
	}
	if ultimo.Etapa != "instalando" || ultimo.Fracao < 0.95 {
		t.Fatalf("último instalando = %+v", ultimo)
	}
}

func TestInstalarNoComputadorCancelar(t *testing.T) {
	a := novoAmbiente(t)
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	trocarInstalar(t, func(ctx context.Context, _ instalacao.Opcoes, _ func(int64, int64)) error {
		cancelar()
		return ctx.Err()
	})
	err := InstalarNoComputador(ctx, a.opcoesComputador(), nil)
	if got := fraseDe(t, err); got != "Instalação cancelada." {
		t.Fatalf("frase = %q", got)
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
	if got := fraseDe(t, err); got != fraseInternetComputador {
		t.Fatalf("frase = %q", got)
	}
	if strings.Contains(fraseDe(t, err), "Nada foi apagado") {
		t.Fatalf("frase do pen drive no computador: %q", fraseDe(t, err))
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

func TestFrasesDoComputadorSemNadaApagado(t *testing.T) {
	for _, err := range []error{
		baixar.ErrConferencia, manifesto.ErrSemManifesto, preparo.ErrFaltaInternet, errors.New("x"),
	} {
		if f := frasePreparoComputador(err); strings.Contains(f, "Nada foi apagado") {
			t.Errorf("%v: %q", err, f)
		}
	}
	if strings.Contains(fraseAntivirusComputador, "Nada foi apagado") {
		t.Errorf("antivírus: %q", fraseAntivirusComputador)
	}
}

func TestEspacoParaInstalarSemInternet(t *testing.T) {
	a := novoAmbiente(t)
	o := a.opcoesComputador()
	a.srv.Close()
	_, _, err := EspacoParaInstalar(context.Background(), o)
	if got := fraseDe(t, err); got != fraseInternetComputador {
		t.Fatalf("frase = %q", got)
	}
}
