// Lançador da CD2 IA de Bolso: o "IA de Bolso" da raiz do pen drive. Uma
// janelinha mostra a IA acordando e um ícone na barra de menus (Mac) ou perto
// do relógio (Windows) segura a IA enquanto ela está acordada. A lógica mora
// em internal/; aqui só a janela, o ícone e a ponte com a tela.
package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

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

func main() {
	r := acharRaiz()
	app := novaApp(r, lerVersao(versao, r))
	err := wails.Run(&options.App{
		Title:             "CD2 IA de Bolso",
		Width:             largura,
		Height:            altura,
		DisableResize:     true,
		HideWindowOnClose: !escondePeloAntesDeFechar,
		BackgroundColour:  &options.RGBA{R: 0x0d, G: 0x12, B: 0x33, A: 255},
		AssetServer:       &assetserver.Options{Assets: telas},
		OnStartup:         app.iniciar,
		OnBeforeClose:     app.antesDeFechar,
		OnShutdown:        app.encerrar,
		Bind:              []interface{}{app},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               idUnico,
			OnSecondInstanceLaunch: func(options.SecondInstanceData) { go app.chamar() },
		},
	})
	if err != nil {
		// Sem janela a IA nem chegou a subir; o Dormir garante que nada fique para trás.
		app.s.Dormir()
		log.Fatalf("lançador %s: %v", lerVersao(versao, r), err)
	}
}
