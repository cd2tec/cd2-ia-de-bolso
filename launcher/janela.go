package main

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/origem"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/cache"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/sessao"
)

// esperaSair é quanto a janela mostra o "Boa noite!" antes de o app sair.
// esperaSairFalha é o mesmo para o aviso de reiniciar, que é mais longo e
// não pode passar batido.
const (
	esperaSair      = 3 * time.Second
	esperaSairFalha = 15 * time.Second
)

// App é a ponte entre a sessão, a janelinha e o ícone da bandeja. Os métodos
// exportados são os que a tela chama (window.go.main.App).
type App struct {
	s       *sessao.Sessao
	bandeja bandeja
	raiz    string
	acordar func() // sobe a IA; trocado no teste

	mu        sync.Mutex
	ctx       context.Context // do Wails; nil até o OnStartup
	podeSair  bool            // a IA já dormiu: o próximo pedido de saída vale
	encerrado bool            // o Wails já parou: nada de falar com a janela
	avisou    bool            // a notificação da primeira vez já saiu
	trancada  bool            // cópia instalada sem chave: a IA não sobe
	origem    origem.Origem   // de onde a IA roda; decidida no OnStartup
	// trocarPara é o lançador instalado que roda depois do Dormiu (o
	// --assumir); vazio, não há troca.
	trocarPara string
}

// estadoTela é o Estado no formato da tela: chaves em minúsculas e a fase
// com inicial maiúscula, como o tela.js compara. A URL fica de fora: ela
// leva a chave da sessão, que só vai para o navegador. Pendrive decide o
// convite para instalar.
type estadoTela struct {
	Fase     string `json:"fase"`
	Pct      int    `json:"pct"`
	Aviso    string `json:"aviso"`
	Erro     string `json:"erro"`
	Pendrive bool   `json:"pendrive"`
}

var nomesFase = map[sessao.Fase]string{
	sessao.Lendo: "Lendo", sessao.Subindo: "Subindo", sessao.Pronta: "Pronta",
	sessao.Dormindo: "Dormindo", sessao.Dormiu: "Dormiu", sessao.Erro: "Erro",
}

func paraTela(e sessao.Estado, pendrive bool) estadoTela {
	return estadoTela{Fase: nomesFase[e.Fase], Pct: e.Pct, Aviso: e.Aviso, Erro: e.Erro, Pendrive: pendrive}
}

// deveMostrar diz se o estado traz a janela de volta sozinho: um erro nunca
// fica escondido, nem com a janela fechada. Vale também para o Dormiu que
// não conseguiu desligar tudo.
func deveMostrar(e sessao.Estado) bool {
	return e.Fase == sessao.Erro || (e.Fase == sessao.Dormiu && e.Erro != "")
}

// esperaAntesDeSair é quanto o Dormiu fica na tela antes de o app sair.
func esperaAntesDeSair(e sessao.Estado) time.Duration {
	if e.Erro != "" {
		return esperaSairFalha
	}
	return esperaSair
}

// fecharVaiDormir diz se fechar a janela (o X) põe a IA para dormir, como o
// botão Fechar. Com o erro na tela não há IA acordada para ficar no ícone, e
// esconder deixaria o problema invisível.
func fecharVaiDormir(f sessao.Fase) bool { return f == sessao.Erro }

// deveChamar diz se mostrar a janela vale um "Me chamou?". Carregando, ela
// só volta mostrando onde o carregamento está.
func deveChamar(f sessao.Fase) bool { return f == sessao.Pronta }

// novaApp prepara o app. Com --inicio a conversa não abre e o aviso do
// primeiro fechar não sai: quem acordou com o computador não pediu janela.
func novaApp(raiz, versao string, args Args) *App {
	a := &App{raiz: raiz, avisou: args.Inicio}
	d := sessao.DepsReais(raiz)
	d.SemNavegador = args.Inicio
	a.s = sessao.Nova(d, versao, a.aoMudar)
	a.acordar = func() { go a.s.Rodar(context.Background()) }
	return a
}

// contexto devolve o ctx do Wails, ou nil se a janela ainda não existe ou já parou.
func (a *App) contexto() context.Context {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.encerrado {
		return nil
	}
	return a.ctx
}

// aoMudar recebe cada estado da sessão, na goroutine de entrega dela: não
// pode chamar o Dormir síncrono aqui.
func (a *App) aoMudar(e sessao.Estado) {
	if ctx := a.contexto(); ctx != nil {
		wruntime.EventsEmit(ctx, "estado", paraTela(e, a.pendrive()))
	}
	a.atualizarBandeja(e)
	if deveMostrar(e) {
		a.mostrar()
	}
	if e.Fase == sessao.Dormiu {
		// Passando a vez para a instalada: ela espera este processo sair,
		// então nada de esperar o "Boa noite!".
		if a.trocar(e.Erro) {
			go a.sair()
			return
		}
		espera := esperaAntesDeSair(e)
		go func() {
			time.Sleep(espera)
			a.sair()
		}()
	}
}

// sair libera a saída e pede ao Wails para fechar. Só depois do Dormiu.
func (a *App) sair() {
	a.mu.Lock()
	a.podeSair = true
	a.mu.Unlock()
	if ctx := a.contexto(); ctx != nil {
		wruntime.Quit(ctx)
	}
}

// iniciar decide a trava antes de subir a IA: a cópia instalada sem chave
// fica na tela Trancada, sem Rodar, e o ícone só mostra ou fecha. A origem
// fica guardada para o convite.
func (a *App) iniciar(ctx context.Context) {
	o := detectarOrigem(a.raiz)
	tranca := deveTrancar(o, func() bool { return chaveValida(pastaCache()) })
	a.mu.Lock()
	a.ctx = ctx
	a.trancada = tranca
	a.origem = o
	a.mu.Unlock()
	iniciarBandeja(a)
	if tranca {
		wruntime.EventsEmit(ctx, "estado", estadoTela{Fase: faseTrancada})
		return
	}
	a.acordar()
}

// antesDeFechar é todo pedido de saída que não veio do Dormiu (Cmd+Q, logout,
// o X da janela no Windows). A IA dorme antes; a saída de verdade vem pelo
// aoMudar, 3 s depois do Dormiu. No Windows o X só esconde, menos com o erro
// na tela: aí vale o Fechar, sem o balão de "Continuo acordada". Trancada
// não há IA para dormir: a saída vale na hora.
func (a *App) antesDeFechar(ctx context.Context) (impedir bool) {
	a.mu.Lock()
	pode := a.podeSair || a.trancada
	a.mu.Unlock()
	if pode {
		return false
	}
	if escondePeloAntesDeFechar {
		if fecharVaiDormir(a.s.Atual().Fase) {
			a.Fechar()
		} else {
			a.esconder()
		}
		return true
	}
	a.mostrar()
	a.Dormir()
	return true
}

// janelaFechada é o X da janela no Mac, que o Wails trata escondendo o app
// (HideWindowOnClose): com o erro na tela ou trancada, vale o Fechar.
func (a *App) janelaFechada() {
	if a.fecharSai() {
		a.Fechar()
	}
}

// esconder é fechar a janela sem parar a IA. No Windows, a primeira vez avisa
// que ela continua perto do relógio.
func (a *App) esconder() {
	if ctx := a.contexto(); ctx != nil {
		wruntime.WindowHide(ctx)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	// Só conta como avisado se a notificação saiu de fato (o ícone pode
	// ainda não existir); senão tenta de novo no próximo fechar.
	if !a.avisou && avisarPrimeiroFechar() {
		a.avisou = true
	}
}

// encerrar é o último passo: o Wails já parou. A IA dorme (se ainda não
// dormiu, espera aqui mesmo, é a última chance) e o ícone some.
func (a *App) encerrar(context.Context) {
	a.mu.Lock()
	a.encerrado = true
	a.mu.Unlock()
	a.s.Dormir()
	fecharBandeja()
}

// mostrar traz a janela, sem "Me chamou?".
func (a *App) mostrar() {
	ctx := a.contexto()
	if ctx == nil {
		return
	}
	wruntime.WindowShow(ctx)
	wruntime.WindowUnminimise(ctx)
	trazerParaFrente()
}

// chamar é o "Mostrar" do menu, o clique simples no Windows e a segunda
// cópia: traz a janela e, com a IA acordada, emite a chamada.
func (a *App) chamar() {
	a.mostrar()
	if ctx := a.contexto(); ctx != nil && deveChamar(a.s.Atual().Fase) {
		wruntime.EventsEmit(ctx, "chamada")
	}
}

// AbrirConversa abre outra janela da conversa no navegador.
func (a *App) AbrirConversa() error { return a.s.AbrirConversa() }

// Dormir põe a IA para dormir e volta na hora. O app sai 3 s depois do Dormiu.
func (a *App) Dormir() { go a.s.Dormir() }

// Fechar é o botão da tela de erro: dorme (parando o que tiver subido) e sai.
// Trancada, nada subiu: sai direto, sem o "Boa noite! Pode tirar o pen drive".
func (a *App) Fechar() {
	if a.estaTrancada() {
		go a.sair()
		return
	}
	a.Dormir()
}

// Estado é o estado de agora, que a tela pede ao abrir (o evento pode ter
// saído antes de a página carregar).
func (a *App) Estado() estadoTela {
	if a.estaTrancada() {
		return estadoTela{Fase: faseTrancada}
	}
	return paraTela(a.s.Atual(), a.pendrive())
}

// AbrirRegistros abre a pasta dos logs no Finder ou no Explorer.
func (a *App) AbrirRegistros() error {
	home, _ := os.UserHomeDir()
	dir := cache.PastaLogs(runtime.GOOS, os.Getenv, home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	nome := "open"
	if runtime.GOOS == "windows" {
		nome = "explorer.exe"
	}
	cmd := exec.Command(nome, dir)
	if err := cmd.Start(); err != nil {
		return err
	}
	// O explorer.exe sai com código 1 mesmo quando abre: só colhe o processo.
	go func() { _ = cmd.Wait() }()
	return nil
}
