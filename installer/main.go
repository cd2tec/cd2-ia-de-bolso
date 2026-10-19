// Instalador da CD2 IA de Bolso: uma janela que grava o pen drive em quatro
// telas. A lógica mora em internal/; aqui só a janela e a ponte com a tela.
package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/webview2"
)

//go:embed all:frontend/dist
var telas embed.FS

// versao é trocada no build com -ldflags "-X main.versao=...".
var versao = "dev"

func main() {
	if servirAjudante(os.Args[1:]) {
		return
	}
	app := NovaApp()
	app.iniciarComArgs(os.Args[1:])
	err := wails.Run(&options.App{
		Title:            "CD2 IA de Bolso",
		Width:            560,
		Height:           420,
		DisableResize:    true,
		BackgroundColour: &options.RGBA{R: 242, G: 242, B: 242, A: 255},
		AssetServer:      &assetserver.Options{Assets: telas},
		OnStartup:        app.iniciar,
		OnBeforeClose:    app.antesDeFechar,
		OnShutdown:       app.encerrar,
		Bind:             []interface{}{app},
		// Fora da Roaming: em rede de empresa ela pode não aceitar escrita.
		Windows: &windows.Options{WebviewUserDataPath: webview2.Pasta("instalador")},
	})
	if err != nil {
		log.Fatalf("instalador %s: %v", versao, err)
	}
}
