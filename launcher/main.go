// Lançador da CD2 IA de Bolso: o "IA de Bolso" da raiz do pen drive. Uma
// janelinha mostra a IA acordando e um ícone na barra de menus (Mac) ou perto
// do relógio (Windows) segura a IA enquanto ela está acordada. A lógica mora
// em internal/; aqui só a janela, o ícone e a ponte com a tela.
package main

import (
	"embed"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/webview2"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/cache"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/raiz"
)

//go:embed all:frontend/dist
var telas embed.FS

// versao é trocada no build com -ldflags "-X main.versao=...". Vazia, vale a
// do VERSION da raiz do pen drive.
var versao = ""

// idUnico é o mesmo em todo lançador: uma IA por computador, mesmo entre
// pen drives diferentes.
const idUnico = "br.com.cd2.iadebolso.lancador"

// Tamanho da janelinha. A altura cabe a tela mais alta (Pronta com o aviso do
// banco, uns 340 px de conteúdo); no Windows ela inclui a barra de título.
const largura, altura = 320, 380

// lerVersao dá preferência ao -ldflags; sem ele, lê <raiz>/VERSION.
func lerVersao(ld, dir string) string {
	if ld != "" {
		return ld
	}
	if b, err := os.ReadFile(filepath.Join(dir, "VERSION")); err == nil {
		if v := strings.TrimSpace(string(b)); v != "" {
			return v
		}
	}
	return "dev"
}

// acharRaiz devolve a raiz do pen drive. Fora de um .app (go run, testes),
// cai na pasta do executável: a sessão então mostra o erro de modelo ausente.
func acharRaiz() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	if r, err := raiz.DoExecutavel(exe, runtime.GOOS); err == nil {
		return r
	}
	return filepath.Dir(exe)
}

// dormirNoSinal põe a IA para dormir num kill (TERM) ou Ctrl+C (INT) antes
// de sair: sem isso o motor e o cd2-ctl ficariam órfãos.
func dormirNoSinal(a *App) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, os.Interrupt)
	go func() {
		<-c
		a.s.Dormir()
		fecharBandeja()
		os.Exit(0)
	}()
}

func main() {
	args := lerArgs(os.Args[1:])
	if args.Esperar > 0 {
		// O lançador do pen drive passou a vez e está saindo: só depois dele
		// a trava de cópia única fica livre.
		esperarProcesso(args.Esperar, prazoEsperar, 100*time.Millisecond, processoVivo)
	}
	r := acharRaiz()
	app := novaApp(r, lerVersao(versao, r), args)
	dormirNoSinal(app)
	err := wails.Run(&options.App{
		Title:             "CD2 IA de Bolso",
		Width:             largura,
		Height:            altura,
		DisableResize:     true,
		HideWindowOnClose: !escondePeloAntesDeFechar,
		StartHidden:       args.Inicio, // acordou com o computador: só o ícone
		BackgroundColour:  &options.RGBA{R: 0x0d, G: 0x12, B: 0x33, A: 255},
		AssetServer:       &assetserver.Options{Assets: telas},
		OnStartup:         app.iniciar,
		OnBeforeClose:     app.antesDeFechar,
		OnShutdown:        app.encerrar,
		Bind:              []interface{}{app},
		Windows: &windows.Options{
			Messages:            mensagensWebView2(),
			WebviewUserDataPath: webview2.Pasta("lancador"),
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: idUnico,
			// Args vem sem o nome do programa (os.Args[1:] da outra cópia).
			OnSecondInstanceLaunch: func(d options.SecondInstanceData) { go app.segundaCopia(d.Args) },
		},
	})
	if err != nil {
		// Sem janela a IA nem chegou a subir; o Dormir garante que nada fique para trás.
		app.s.Dormir()
		home, _ := os.UserHomeDir()
		v := lerVersao(versao, r)
		if errLog := registrarFalha(cache.PastaLogs(runtime.GOOS, os.Getenv, home), v, err); errLog != nil {
			log.Printf("não consegui registrar a falha: %v", errLog)
		}
		log.Fatalf("lançador %s: %v", v, err)
	}
}
