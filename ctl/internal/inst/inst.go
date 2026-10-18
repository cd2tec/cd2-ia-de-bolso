// Package inst guarda o estado da instalação paga (progresso, chave, abrir a IA instalada) para
// as rotas /instalacao do cd2-ctl. A cópia em si é do pacote instalacao.
package inst

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/chave"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/codigo"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/copiar"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/guarda"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/origem"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/sistema"
)

// Erros que a API traduz em código HTTP. Os textos vão direto para a tela.
var (
	ErrOcupada      = errors.New("Já tem uma instalação em andamento.")
	ErrChaveNaoVale = errors.New("Esta chave não é deste computador.")
	ErrNaoInstalada = errors.New("A IA ainda não foi instalada.")
	ErrSemChave     = instalacao.ErrSemChave
	// ErrSoPendrive: instalar e gravar chave só valem quando a IA roda do pen drive.
	ErrSoPendrive = errors.New("A instalação é feita pelo pen drive.")
	// ErrSemCodigo: a identidade deste computador não pôde ser lida; a culpa não é da chave.
	ErrSemCodigo = errors.New("Não consegui ler o código deste computador. Tente de novo.")
)

const msgCopiaRuim = "Não consegui copiar direito. Tente de novo."

// Fases do progresso.
const (
	Parada   = "parada"
	Copiando = "copiando"
	Pronta   = "pronta"
	Erro     = "erro"
)

// Info é a resposta de GET /instalacao.
type Info struct {
	Origem    string `json:"origem"`
	Codigo    string `json:"codigo"`
	TemChave  bool   `json:"temChave"`
	Instalada bool   `json:"instalada"`
}

// Progresso é a resposta de GET /instalacao/progresso.
type Progresso struct {
	Fase   string `json:"fase"`
	Feitos int64  `json:"feitos"`
	Total  int64  `json:"total"`
	Erro   string `json:"erro,omitempty"`
	Aviso  string `json:"aviso,omitempty"`
}

// Em variáveis para os testes trocarem a máquina de verdade.
var (
	instalarFn     = instalacao.Instalar
	valida         = guarda.Valida
	lerCodigo      = codigo.Ler
	lancadorExiste = func(p string) bool { _, err := os.Stat(p); return err == nil }
	iniciar        = iniciarDestacado
	detectar       = origem.Detectar
	destinoFn      = func() string {
		home, _ := os.UserHomeDir()
		return sistema.Destino(runtime.GOOS, os.Getenv, home)
	}
)

// Backend guarda o progresso da instalação. Seguro para uso concorrente.
type Backend struct {
	raiz, cache string
	ctx         context.Context
	cancel      context.CancelFunc

	mu   sync.Mutex
	prog Progresso
}

// New monta o Backend com a raiz do pen drive e a pasta de cache (--root e --cache).
func New(raiz, cache string) *Backend {
	ctx, cancel := context.WithCancel(context.Background())
	return &Backend{raiz: raiz, cache: cache, ctx: ctx, cancel: cancel, prog: Progresso{Fase: Parada}}
}

// Shutdown cancela uma instalação em andamento.
func (b *Backend) Shutdown() { b.cancel() }

func (b *Backend) lancador() string { return sistema.Lancador(destinoFn(), runtime.GOOS) }

// Info diz de onde a IA roda, o código deste computador e se já há chave e instalação.
func (b *Backend) Info() Info {
	cod, err := lerCodigo()
	org := string(detectar(b.raiz))
	if err != nil {
		cod, org = "", string(origem.Desconhecida)
	}
	return Info{
		Origem:    org,
		Codigo:    cod,
		TemChave:  valida(b.cache),
		Instalada: lancadorExiste(b.lancador()),
	}
}

// GravarChave confere a chave contra o código deste computador e só então a grava.
func (b *Backend) GravarChave(k string) error {
	if detectar(b.raiz) != origem.Pendrive {
		return ErrSoPendrive
	}
	norm, ok := chave.Normalizar(k)
	if !ok {
		return ErrChaveNaoVale
	}
	cod, err := lerCodigo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cd2-ctl: nao consegui ler o codigo deste computador: %v\n", err)
		return ErrSemCodigo
	}
	if !chave.Conferir(norm, cod) {
		return ErrChaveNaoVale
	}
	if err := guarda.Gravar(b.cache, norm); err != nil {
		fmt.Fprintf(os.Stderr, "cd2-ctl: nao consegui gravar a chave: %v\n", err)
		return errors.New(msgCopiaRuim)
	}
	return nil
}

// Progresso devolve uma cópia do estado atual.
func (b *Backend) Progresso() Progresso {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.prog
}

// Instalar começa a copiar em segundo plano. ErrSoPendrive fora do pen drive; ErrSemChave se não
// há chave válida; ErrOcupada se já há uma cópia rodando.
func (b *Backend) Instalar(areaDeTrabalho, inicio bool) error {
	if detectar(b.raiz) != origem.Pendrive {
		return ErrSoPendrive
	}
	if !valida(b.cache) {
		return ErrSemChave
	}
	b.mu.Lock()
	if b.prog.Fase == Copiando {
		b.mu.Unlock()
		return ErrOcupada
	}
	b.prog = Progresso{Fase: Copiando}
	b.mu.Unlock()
	versao := ""
	if v, err := os.ReadFile(filepath.Join(b.raiz, "VERSION")); err == nil {
		versao = strings.TrimSpace(string(v))
	}
	o := instalacao.Opcoes{Raiz: b.raiz, Cache: b.cache, Versao: versao, AreaDeTrabalho: areaDeTrabalho, Inicio: inicio}
	go b.rodar(o)
	return nil
}

func (b *Backend) rodar(o instalacao.Opcoes) {
	err := instalarFn(b.ctx, o, func(feitos, total int64) {
		b.mu.Lock()
		b.prog.Feitos, b.prog.Total = feitos, total
		b.mu.Unlock()
	})
	b.mu.Lock()
	defer b.mu.Unlock()
	switch {
	case err == nil:
		b.prog.Fase = Pronta
	case errors.Is(err, instalacao.ErrAvisoAtalhos):
		b.prog.Fase, b.prog.Aviso = Pronta, instalacao.ErrAvisoAtalhos.Error()
	default:
		fmt.Fprintf(os.Stderr, "cd2-ctl: instalacao falhou: %v\n", err)
		b.prog.Fase, b.prog.Erro = Erro, textoDoErro(err)
	}
}

func textoDoErro(err error) string {
	for _, s := range []error{copiar.ErrSemEspaco, copiar.ErrEmUso, copiar.ErrCopiaRuim, instalacao.ErrSemChave} {
		if errors.Is(err, s) {
			return s.Error()
		}
	}
	return msgCopiaRuim
}

// Abrir roda o lançador instalado com --assumir, sem esperar por ele.
func (b *Backend) Abrir() error {
	l := b.lancador()
	if !lancadorExiste(l) {
		return ErrNaoInstalada
	}
	if err := iniciar(l, "--assumir"); err != nil {
		fmt.Fprintf(os.Stderr, "cd2-ctl: nao consegui abrir %s: %v\n", l, err)
		return errors.New("Não consegui abrir a IA instalada.")
	}
	return nil
}
