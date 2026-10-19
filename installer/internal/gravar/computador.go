package gravar

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/copiar"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/sistema"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/manifesto"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/pendrive"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/preparo"
)

// Frases do caminho de instalar no computador.
const (
	fraseInstalando         = "Instalando no computador"
	fraseInstalouOk         = "Pronto. A IA de Bolso está instalada neste computador."
	fraseInstalacaoCancelou = "Instalação cancelada."
)

// margemInstalacao é o que a instalação ocupa além do download, e
// folgaInstalacao é a folga sobre o total.
const (
	margemInstalacao = 3_000_000_000
	folgaInstalacao  = 1.1
)

// Em variáveis para os testes trocarem a máquina de verdade.
var (
	instalarFn     = instalacao.Instalar
	espacoLivreFn  = copiar.EspacoLivre
	pastaDestinoFn = func() string {
		home, _ := os.UserHomeDir()
		return sistema.Destino(runtime.GOOS, os.Getenv, home)
	}
)

// errosComFrase são os erros da instalação cujo texto já é a frase da tela.
var errosComFrase = []error{
	instalacao.ErrSemChave,
	copiar.ErrSemEspaco, copiar.ErrEmUso, copiar.ErrCopiaRuim,
	sistema.ErrSistema, sistema.ErrSemPastaPessoal,
}

// OpcoesComputador reúne o que InstalarNoComputador precisa. Cache é a pasta
// do stage (a do pen drive, cd2-harness/usb); CacheChave é a pasta da chave
// (cd2-harness), o instalacao.Opcoes.Cache.
type OpcoesComputador struct {
	Cache, CacheChave, URLManifesto string
	HTTP                            *http.Client
	AreaDeTrabalho, Inicio          bool
	Agora                           func() time.Time // relógio; time.Now quando nil
}

func (o OpcoesComputador) url() string {
	if o.URLManifesto == "" {
		return URLManifesto
	}
	return o.URLManifesto
}

// EspacoParaInstalar diz quanto a instalação precisa (o que falta baixar mais
// uns 3 GB da cópia, com 10% de folga) e quanto há livre no disco do destino.
func EspacoParaInstalar(ctx context.Context, o OpcoesComputador) (precisa, livre int64, err error) {
	m, _, err := manifesto.Obter(ctx, o.HTTP, o.url(), o.Cache)
	if err != nil {
		return 0, 0, &Erro{Frase: frasePreparo(err), Causa: err}
	}
	precisa = int64(float64(preparo.BytesParaBaixar(m, o.Cache)+margemInstalacao) * folgaInstalacao)
	livre, err = espacoLivreFn(pastaQueExiste(filepath.Dir(pastaDestinoFn())))
	if err != nil {
		return 0, 0, &Erro{Frase: fraseGenerica, Causa: err}
	}
	return precisa, livre, nil
}

// pastaQueExiste sobe a partir de dir até a primeira pasta que existe.
func pastaQueExiste(dir string) string {
	for {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
		pai := filepath.Dir(dir)
		if pai == dir {
			return dir
		}
		dir = pai
	}
}

// InstalarNoComputador baixa e confere o stage, como o Gravar, e o entrega
// ao instalacao.Instalar. Todo erro devolvido é *Erro. Atalho ou início que
// falhou não é erro: o "pronto" leva o aviso.
func InstalarNoComputador(ctx context.Context, o OpcoesComputador, ev func(Evento)) error {
	if ev == nil {
		ev = func(Evento) {}
	}
	agora := o.Agora
	if agora == nil {
		agora = time.Now
	}
	falha := func(frase string, causa error) error {
		if ctx.Err() != nil {
			return &Erro{Frase: fraseInstalacaoCancelou, Causa: causa}
		}
		return &Erro{Frase: frase, Causa: causa}
	}

	m, daCache, err := manifesto.Obter(ctx, o.HTTP, o.url(), o.Cache)
	if err != nil {
		return falha(frasePreparo(err), err)
	}

	a := &andamento{agora: agora, ev: ev, copiaUmaVez: true}
	a.baixar = preparo.BytesParaBaixar(m, o.Cache)
	// Antes de ver o stage, estima a cópia pelos tamanhos que o manifesto conhece.
	a.copiar = m.Zip.Bytes + m.Llamafile.Bytes + m.Modelo.Bytes
	if a.copiar < a.baixar {
		a.copiar = a.baixar
	}

	itens := map[string]int64{}
	etapa := "baixando"
	stage, err := preparo.Montar(ctx, o.HTTP, m, o.Cache, daCache, func(e preparo.Evento) {
		if !e.Conferindo {
			itens[e.Item] = e.Feitos
		}
		var baixados int64
		for _, v := range itens {
			baixados += v
		}
		if baixados > a.baixar {
			baixados = a.baixar
		}
		frase := fraseConferindoArquivos
		if !e.Conferindo {
			frase = fraseBaixando(e.Item, e.Feitos, e.Total)
		}
		// Com o grosso baixado, a etapa passa a "conferindo" e não volta:
		// o preparo alterna conferir e baixar a cada item, e a tela não pisca.
		if e.Conferindo && baixados >= a.baixar {
			etapa = "conferindo"
		}
		a.publicar(etapa, frase, baixados, a.baixar, 0, false)
	})
	if err != nil {
		return falha(frasePreparo(err), err)
	}

	if sistemaOperacional == "windows" {
		exe := filepath.Join(stage, "bin", "llamafile.exe")
		if st, err := os.Stat(exe); err != nil || st.Size() == 0 {
			log.Printf("instalar: %s sumiu ou zerou (antivírus?), pasta para liberar: %s", exe, o.Cache)
			return falha(fraseAntivirus, fmt.Errorf("llamafile.exe sumiu ou zerou: %v", err))
		}
	}

	total, err := pendrive.TamanhoTotal(stage)
	if err != nil {
		return falha(fraseGenerica, err)
	}
	a.copiar = total
	a.publicar("instalando", fraseInstalando, 0, total, a.baixar, true)
	// O total da cópia (só os arquivos deste computador) é menor que o stage:
	// a conta é proporcional, para a barra chegar ao fim junto com a cópia.
	prog := func(f, t int64) {
		if t > 0 {
			f = int64(float64(f) / float64(t) * float64(total))
		}
		a.publicar("instalando", fraseInstalando, f, total, a.baixar, false)
	}
	err = instalarFn(ctx, instalacao.Opcoes{
		Raiz: stage, Cache: o.CacheChave, Versao: m.Versao,
		AreaDeTrabalho: o.AreaDeTrabalho, Inicio: o.Inicio,
	}, prog)
	switch {
	case err == nil:
		a.pronto(fraseInstalouOk)
		return nil
	case errors.Is(err, instalacao.ErrAvisoAtalhos):
		log.Printf("instalar: %v", err)
		a.pronto(instalacao.ErrAvisoAtalhos.Error())
		return nil
	}
	return falha(fraseInstalacao(err), err)
}

// fraseInstalacao traduz um erro do instalacao.Instalar.
func fraseInstalacao(err error) string {
	for _, e := range errosComFrase {
		if errors.Is(err, e) {
			return e.Error()
		}
	}
	return fraseGenerica
}
