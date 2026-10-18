package inst

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/copiar"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/guarda"
)

func esperaFase(t *testing.T, b *Backend, fase string) Progresso {
	t.Helper()
	for i := 0; i < 400; i++ {
		if p := b.Progresso(); p.Fase == fase {
			return p
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("fase %q não chegou; última: %+v", fase, b.Progresso())
	return Progresso{}
}

func novo(t *testing.T, f func(context.Context, instalacao.Opcoes, func(int64, int64)) error) *Backend {
	t.Helper()
	raiz := t.TempDir()
	if err := os.WriteFile(filepath.Join(raiz, "VERSION"), []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	oi, ov := instalarFn, valida
	t.Cleanup(func() { instalarFn, valida = oi, ov })
	instalarFn = f
	valida = func(string) bool { return true }
	b := New(raiz, t.TempDir())
	t.Cleanup(b.Shutdown)
	return b
}

func TestProgressoVaiDeCopiandoAPronta(t *testing.T) {
	passo, fim := make(chan struct{}), make(chan struct{})
	var got instalacao.Opcoes
	b := novo(t, func(_ context.Context, o instalacao.Opcoes, prog func(int64, int64)) error {
		got = o
		prog(10, 100)
		close(passo)
		<-fim
		prog(100, 100)
		return nil
	})
	if p := b.Progresso(); p.Fase != "parada" {
		t.Fatalf("antes: %+v", p)
	}
	if err := b.Instalar(true, false); err != nil {
		t.Fatal(err)
	}
	<-passo
	if p := b.Progresso(); p.Fase != "copiando" || p.Feitos != 10 || p.Total != 100 {
		t.Fatalf("copiando: %+v", p)
	}
	if err := b.Instalar(true, false); !errors.Is(err, ErrOcupada) {
		t.Fatalf("segunda: %v", err)
	}
	close(fim)
	p := esperaFase(t, b, "pronta")
	if p.Feitos != 100 || p.Erro != "" || p.Aviso != "" {
		t.Fatalf("pronta: %+v", p)
	}
	if got.Versao != "1.2.3" || !got.AreaDeTrabalho || got.Inicio {
		t.Errorf("opções: %+v", got)
	}
}

func TestAvisoDeAtalhosViraProntaComAviso(t *testing.T) {
	b := novo(t, func(context.Context, instalacao.Opcoes, func(int64, int64)) error {
		return errors.Join(instalacao.ErrAvisoAtalhos, errors.New("raw"))
	})
	if err := b.Instalar(false, false); err != nil {
		t.Fatal(err)
	}
	p := esperaFase(t, b, "pronta")
	if p.Aviso != instalacao.ErrAvisoAtalhos.Error() || p.Erro != "" {
		t.Fatalf("%+v", p)
	}
}

func TestErrosViramTextoEmPortugues(t *testing.T) {
	casos := []struct {
		err  error
		want string
	}{
		{copiar.ErrSemEspaco, copiar.ErrSemEspaco.Error()},
		{copiar.ErrEmUso, copiar.ErrEmUso.Error()},
		{copiar.ErrCopiaRuim, copiar.ErrCopiaRuim.Error()},
		{instalacao.ErrSemChave, instalacao.ErrSemChave.Error()},
		{errors.New("open /x: permission denied"), "Não consegui copiar direito. Tente de novo."},
	}
	for _, c := range casos {
		b := novo(t, func(context.Context, instalacao.Opcoes, func(int64, int64)) error { return c.err })
		if err := b.Instalar(false, false); err != nil {
			t.Fatal(err)
		}
		if p := esperaFase(t, b, "erro"); p.Erro != c.want {
			t.Errorf("%v: %q", c.err, p.Erro)
		}
	}
}

func TestDepoisDoErroPodeTentarDeNovo(t *testing.T) {
	n := 0
	b := novo(t, func(context.Context, instalacao.Opcoes, func(int64, int64)) error {
		n++
		if n == 1 {
			return copiar.ErrSemEspaco
		}
		return nil
	})
	_ = b.Instalar(false, false)
	esperaFase(t, b, "erro")
	if err := b.Instalar(false, false); err != nil {
		t.Fatal(err)
	}
	if p := esperaFase(t, b, "pronta"); p.Erro != "" {
		t.Fatalf("%+v", p)
	}
}

func TestSemChaveNaoComeca(t *testing.T) {
	chamou := false
	b := novo(t, func(context.Context, instalacao.Opcoes, func(int64, int64)) error { chamou = true; return nil })
	valida = func(string) bool { return false }
	if err := b.Instalar(false, false); !errors.Is(err, instalacao.ErrSemChave) {
		t.Fatalf("%v", err)
	}
	if chamou || b.Progresso().Fase != "parada" {
		t.Error("não devia ter começado")
	}
}

func TestShutdownCancelaInstalacao(t *testing.T) {
	cancelou := make(chan struct{})
	b := novo(t, func(ctx context.Context, _ instalacao.Opcoes, _ func(int64, int64)) error {
		<-ctx.Done()
		close(cancelou)
		return ctx.Err()
	})
	_ = b.Instalar(false, false)
	b.Shutdown()
	select {
	case <-cancelou:
	case <-time.After(time.Second):
		t.Fatal("contexto não cancelado")
	}
}

func TestAbrirSemInstalacaoDa409(t *testing.T) {
	b := novo(t, nil)
	oe := lancadorExiste
	t.Cleanup(func() { lancadorExiste = oe })
	lancadorExiste = func(string) bool { return false }
	if err := b.Abrir(); !errors.Is(err, ErrNaoInstalada) {
		t.Fatalf("%v", err)
	}
}

func TestAbrirChamaOLancadorComAssumir(t *testing.T) {
	b := novo(t, nil)
	oe, oi := lancadorExiste, iniciar
	t.Cleanup(func() { lancadorExiste, iniciar = oe, oi })
	lancadorExiste = func(string) bool { return true }
	var caminho string
	var args []string
	iniciar = func(c string, a ...string) error { caminho, args = c, a; return nil }
	if err := b.Abrir(); err != nil {
		t.Fatal(err)
	}
	if caminho == "" || len(args) != 1 || args[0] != "--assumir" {
		t.Fatalf("%q %v", caminho, args)
	}
}

func TestGravarChaveRecusaQuemNaoVale(t *testing.T) {
	b := novo(t, nil)
	if err := b.GravarChave("lixo"); !errors.Is(err, ErrChaveNaoVale) {
		t.Fatalf("%v", err)
	}
	if guarda.Ler(b.cache) != "" {
		t.Error("não devia ter gravado")
	}
}
