package main

import (
	"context"
	"errors"
	"net/http"
	goruntime "runtime"
	"sync"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/chave"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/codigo"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/discos"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/elevacao"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/gravar"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// fraseGenerica aparece quando o erro não traz frase pronta para a pessoa.
const fraseGenerica = "Não deu certo. Tente de novo."

// tempoListar limita uma listagem de discos; a tela tenta de novo logo depois.
const tempoListar = 30 * time.Second

// Fim é o último evento da gravação, como a tela recebe. Aviso só vem na
// instalação no computador que terminou, mas com um porém (os atalhos).
type Fim struct {
	Ok    bool   `json:"ok"`
	Frase string `json:"frase"`
	Aviso string `json:"aviso,omitempty"`
}

// App é a ponte entre a janela e os pacotes do instalador. Os métodos
// exportados viram window.go.main.App.* na tela.
type App struct {
	ctx     context.Context
	sistema discos.Sistema

	// Trocáveis no teste.
	emitir     func(ctx context.Context, nome string, dados ...any)
	gravar     func(ctx context.Context, o gravar.Opcoes, ev func(gravar.Evento)) error
	pastaCache func() (string, error)

	// Trocáveis no teste, do caminho de instalar no computador.
	instalar      func(ctx context.Context, o gravar.OpcoesComputador, ev func(gravar.Evento)) error
	espaco        func(ctx context.Context, o gravar.OpcoesComputador) (precisa, livre int64, err error)
	pastaChave    func() (string, error)
	lerCodigo     func() (string, error)
	conferir      func(chave, codigo string) bool
	abrirURL      func(ctx context.Context, url string)
	sair          func(ctx context.Context)
	elevado       func() bool
	reabrir       func(exe string, args []string) error
	novoSinal     func() (string, error)
	esperarSinal  func(nonce string) bool
	sinalizar     func(nonce string) error
	abrirLancador func(caminho string) error

	comInstalar bool // abriu com --instalar

	mu         sync.Mutex
	cancelar   context.CancelFunc // não nil enquanto uma gravação ou instalação roda
	instalando bool               // o que roda é a instalação no computador
	bloqueio   string             // não deu para reabrir sem administrador
	voltas     []func()           // desligam os servidores da volta do pagamento, do mais antigo ao mais novo
}

// NovaApp monta a App com o sistema e as funções de verdade.
func NovaApp() *App {
	return &App{
		ctx:        context.Background(),
		sistema:    sistemaPadrao(),
		emitir:     runtime.EventsEmit,
		gravar:     gravar.Gravar,
		pastaCache: gravar.PastaCache,

		instalar:      gravar.InstalarNoComputador,
		espaco:        gravar.EspacoParaInstalar,
		pastaChave:    pastaChavePadrao,
		lerCodigo:     codigo.Ler,
		conferir:      chave.Conferir,
		abrirURL:      runtime.BrowserOpenURL,
		sair:          runtime.Quit,
		elevado:       elevacao.Elevado,
		reabrir:       elevacao.ReabrirSemAdmin,
		novoSinal:     elevacao.NovoSinal,
		esperarSinal:  func(n string) bool { return elevacao.EsperarSinal(n, elevacao.PrazoSinal) },
		sinalizar:     elevacao.Sinalizar,
		abrirLancador: abrirSolto,
	}
}

// iniciar guarda o contexto da janela, que os eventos exigem.
func (a *App) iniciar(ctx context.Context) { a.ctx = ctx }

// ListarDiscos devolve os discos que a Tela 1 mostra.
func (a *App) ListarDiscos() ([]discos.Disco, error) {
	ctx, cancel := context.WithTimeout(a.ctx, tempoListar)
	defer cancel()
	return gravar.Discos(ctx, a.sistema)
}

// Gravar começa a gravação e devolve logo. O andamento chega pelo evento
// "progresso" e o resultado pelo evento "fim".
func (a *App) Gravar(d discos.Disco) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancelar != nil {
		return errors.New("Já tem uma gravação em andamento.")
	}
	cache, err := a.pastaCache()
	if err != nil {
		return errors.New(fraseGenerica)
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancelar = cancel
	a.instalando = false
	o := gravar.Opcoes{
		Disco:        d,
		Cache:        cache,
		URLManifesto: gravar.URLManifesto,
		// Sem timeout total: o download do modelo leva muitos minutos.
		HTTP:    &http.Client{},
		Sistema: a.sistema,
	}
	go a.rodar(ctx, cancel, func(ctx context.Context, ev func(gravar.Evento)) error {
		return a.gravar(ctx, o, ev)
	})
	return nil
}

// rodar roda uma gravação ou instalação, emite "progresso" a cada evento e
// "fim" no final.
func (a *App) rodar(ctx context.Context, cancel context.CancelFunc, trabalho func(context.Context, func(gravar.Evento)) error) {
	var ultimo gravar.Evento
	err := trabalho(ctx, func(e gravar.Evento) {
		ultimo = e
		a.emitir(a.ctx, "progresso", e)
	})
	fim := Fim{Ok: err == nil}
	if err != nil {
		var ge *gravar.Erro
		if errors.As(err, &ge) && ge.Frase != "" {
			fim.Frase = ge.Error()
		} else {
			fim.Frase = fraseGenerica
		}
	} else if ultimo.Etapa == "pronto" {
		fim.Frase = ultimo.Frase
		// A instalação deu certo, mas sem todos os atalhos: a tela avisa.
		if ultimo.Frase == instalacao.ErrAvisoAtalhos.Error() {
			fim.Aviso = ultimo.Frase
		}
	}
	// Libera antes de avisar a tela, para "Gravar outro" funcionar na hora.
	a.mu.Lock()
	a.cancelar = nil
	a.mu.Unlock()
	cancel()
	a.emitir(a.ctx, "fim", fim)
}

// Gravando diz se há uma gravação em andamento.
func (a *App) Gravando() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cancelar != nil
}

// antesDeFechar roda quando a pessoa tenta fechar a janela. Devolve true para
// impedir o fechamento. Com gravação em andamento, pergunta antes; se a pessoa
// insistir, cancela a gravação e deixa fechar.
func (a *App) antesDeFechar(ctx context.Context) bool {
	a.mu.Lock()
	rodando, instalando := a.cancelar != nil, a.instalando
	a.mu.Unlock()
	if !rodando {
		return false
	}
	continuar := "Continuar gravando"
	mensagem := "A gravação ainda não terminou. Se fechar agora, o pen drive pode ficar incompleto. Fechar mesmo assim?"
	if instalando {
		continuar = "Continuar instalando"
		mensagem = "A instalação ainda não terminou. Fechar mesmo assim?"
	}
	// No Windows o Wails ignora os rótulos e mostra Sim/Não (devolve "Yes"/"No"),
	// com "No" como padrão. No Mac valem os rótulos.
	padrao := continuar
	if goruntime.GOOS == "windows" {
		padrao = "No"
	}
	r, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "CD2 IA de Bolso",
		Message:       mensagem,
		Buttons:       []string{continuar, "Fechar"},
		DefaultButton: padrao,
		CancelButton:  padrao,
	})
	if err != nil || (r != "Fechar" && r != "Yes") {
		return true
	}
	a.Cancelar()
	return false
}

// Cancelar interrompe a gravação em andamento, se houver.
func (a *App) Cancelar() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancelar != nil {
		a.cancelar()
	}
}
