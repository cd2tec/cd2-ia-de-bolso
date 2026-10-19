package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/cache"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// abrirPeloBat é a saída quando a janelinha não abre no Windows: o .bat em
// core\ faz o mesmo pelo terminal, sem WebView2.
const abrirPeloBat = "Dá para abrir a IA pelo INICIAR-CD2-Windows.bat, na pasta core do pen drive."

// mensagensWebView2 são as caixas de aviso que o Wails mostra no Windows
// quando falta o WebView2 (o componente da Microsoft que desenha a janelinha)
// ou quando ele falha. Sem isto saem em inglês.
func mensagensWebView2() *windows.Messages {
	return &windows.Messages{
		InstallationRequired: "A janelinha da IA de Bolso precisa do WebView2, um componente gratuito da Microsoft que não está neste computador. " +
			"Clique em OK para baixar e instalar. Pode levar alguns minutos, e nada aparece na tela enquanto baixa. " + abrirPeloBat,
		UpdateRequired: "A janelinha da IA de Bolso precisa de uma versão mais nova do WebView2, um componente gratuito da Microsoft. " +
			"Clique em OK para baixar e atualizar. Pode levar alguns minutos, e nada aparece na tela enquanto baixa. " + abrirPeloBat,
		MissingRequirements:  "Falta um componente",
		Webview2NotInstalled: "O WebView2 não está instalado, e sem ele a janelinha não abre. " + abrirPeloBat,
		Error:                "Algo deu errado",
		FailedToInstall:      "Não consegui instalar o WebView2. Tente de novo mais tarde. " + abrirPeloBat,
		DownloadPage: "A janelinha da IA de Bolso precisa do WebView2, um componente gratuito da Microsoft. " +
			"Clique em OK para abrir a página de download. Versão mínima: ",
		PressOKToInstall:     " Clique em OK para instalar.",
		ContactAdmin:         "A janelinha da IA de Bolso precisa do WebView2, um componente da Microsoft, e só quem cuida deste computador pode instalá-lo. " + abrirPeloBat,
		InvalidFixedWebview2: "O WebView2 indicado para a janelinha não serve. " + abrirPeloBat,
		WebView2ProcessCrash: "A janelinha da IA de Bolso parou de funcionar e precisa ser aberta de novo. " + abrirPeloBat,
	}
}

// registrarFalha acrescenta a falha ao lancador.log da pasta de logs: sem
// janela, é o único lugar onde fica o motivo.
func registrarFalha(dir, versao string, falha error) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "lancador.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%s lançador %s: %v\n", time.Now().Format(time.RFC3339), versao, falha)
	if errFechar := f.Close(); err == nil {
		err = errFechar
	}
	return err
}

// candidatasWebView2 são as pastas, em ordem de preferência, para os dados do
// WebView2 no Windows. Sem elas o Wails usa %APPDATA%\<nome do exe>, a pasta
// Roaming, que em rede de empresa costuma ser redirecionada ou bloqueada, e aí
// a janelinha não abre ("não pode ler e gravar no diretório de dados").
func candidatasWebView2(env func(string) string, home string) []string {
	var c []string
	if env("LOCALAPPDATA") != "" {
		c = append(c, cache.Pasta("windows", env, home)+`\webview2`)
	}
	if env("TEMP") != "" {
		c = append(c, cache.PastaLogs("windows", env, home)+`\webview2`)
	}
	return c
}

// pastaDadosWebView2 devolve a primeira candidata em que dá para gravar de
// fato (cria a pasta e um arquivo de teste). Vazio se nenhuma serve: o Wails
// fica com o padrão dele, que ainda pode dar certo.
func pastaDadosWebView2(candidatas []string) string {
	for _, dir := range candidatas {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}
		f, err := os.CreateTemp(dir, "grava-*")
		if err != nil {
			continue
		}
		f.Close()
		os.Remove(f.Name())
		return dir
	}
	return ""
}
