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
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/origem"
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
	oi, ov, od, ol := instalarFn, valida, detectar, lerCodigo
	t.Cleanup(func() { instalarFn, valida, detectar, lerCodigo = oi, ov, od, ol })
	instalarFn = f
	valida = func(string) bool { return true }
	detectar = func(string) origem.Origem { return origem.Pendrive }
	lerCodigo = func() (string, error) { return "ABCD-EFGH-JKLM", nil }
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

// A instalação só sai do pen drive: a cópia instalada recusa instalar e gravar chave.
func TestForaDoPenDriveRecusaInstalarEGravarChave(t *testing.T) {
	for _, org := range []origem.Origem{origem.Instalada, origem.Desconhecida} {
		chamou := false
		b := novo(t, func(context.Context, instalacao.Opcoes, func(int64, int64)) error { chamou = true; return nil })
		detectar = func(string) origem.Origem { return org }
		if err := b.Instalar(false, false); !errors.Is(err, ErrSoPendrive) {
			t.Errorf("%s: Instalar = %v", org, err)
		}
		if err := b.GravarChave("qualquer"); !errors.Is(err, ErrSoPendrive) {
			t.Errorf("%s: GravarChave = %v", org, err)
		}
		if chamou || b.Progresso().Fase != Parada || guarda.Ler(b.cache) != "" {
			t.Errorf("%s: não devia ter feito nada", org)
		}
	}
	if ErrSoPendrive.Error() != "A instalação é feita pelo pen drive." {
		t.Errorf("texto: %q", ErrSoPendrive.Error())
	}
}

func TestCodigoIlegivelNaoCulpaAChave(t *testing.T) {
	b := novo(t, nil)
	ol := lerCodigo
	t.Cleanup(func() { lerCodigo = ol })
	lerCodigo = func() (string, error) { return "", errors.New("sem identidade") }
	// Chave bem formada (a do vetor de teste): o erro só pode vir do código.
	err := b.GravarChave("CD2-YH4D1FNNZF24C755M18TR44MVFRF9H1X8EV4JY7GAWV4PS4NFRY24BKFME9PD6HQMANW5RXDRJPCJQ5ZG1001W7W5BNMH71H8JVVA10")
	if err == nil || errors.Is(err, ErrChaveNaoVale) || err.Error() != "Não consegui ler o código deste computador. Tente de novo." {
		t.Fatalf("%v", err)
	}
	if guarda.Ler(b.cache) != "" {
		t.Error("não devia ter gravado")
	}
}

func TestProgressoDizOQueEstaCopiando(t *testing.T) {
	casos := map[string]string{
		"models/modelo.gguf":                   "o modelo de IA",
		"bin/llamafile":                        "o motor",
		"bin/llamafile.exe":                    "o motor",
		"web/index.html":                       "os arquivos da IA",
		"bin/ctl/darwin-arm64/cd2-ctl":         "os arquivos da IA",
		"IA de Bolso.app/Contents/MacOS/bolso": "os arquivos da IA",
	}
	for destino, quer := range casos {
		passo, fim := make(chan struct{}), make(chan struct{})
		b := novo(t, func(_ context.Context, o instalacao.Opcoes, prog func(int64, int64)) error {
			o.AoArquivo(destino)
			prog(1, 10)
			close(passo)
			<-fim
			return nil
		})
		if err := b.Instalar(false, false); err != nil {
			t.Fatal(err)
		}
		<-passo
		if p := b.Progresso(); p.Arquivo != quer {
			t.Errorf("%s: arquivo %q, queria %q", destino, p.Arquivo, quer)
		}
		close(fim)
		esperaFase(t, b, "pronta")
	}
}

// O lançador instalado roda com a pasta dele como diretório atual, não a do cd2-ctl no pen drive.
func TestComandoDestacadoRodaNaPastaDoLancador(t *testing.T) {
	l := filepath.Join(t.TempDir(), "IA de Bolso", "IA de Bolso.exe")
	c := comandoDestacado(l, "--assumir")
	if c.Dir != filepath.Dir(l) {
		t.Errorf("Dir = %q, quero %q", c.Dir, filepath.Dir(l))
	}
	if c.Path != l || len(c.Args) != 2 || c.Args[1] != "--assumir" {
		t.Errorf("comando = %q %v", c.Path, c.Args)
	}
}
