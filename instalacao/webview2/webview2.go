// Package webview2 escolhe a pasta de dados do WebView2 no Windows para o
// lançador e o instalador. Sem ela o Wails usa %APPDATA%\<nome do exe>, a
// pasta Roaming, que em rede de empresa costuma ser redirecionada ou
// bloqueada, e aí a janela não abre ("O Microsoft Edge não pode ler e gravar
// no diretório de dados").
package webview2

import (
	"os"
	"runtime"
)

// Pasta devolve a pasta de dados do WebView2 para o app (cada app tem a sua:
// o WebView2 não aceita dois programas diferentes na mesma pasta). Fora do
// Windows, ou se nenhuma candidata grava, devolve vazio e o Wails fica com o
// padrão dele.
func Pasta(app string) string {
	if runtime.GOOS != "windows" {
		return ""
	}
	return Escolher(Candidatas(os.Getenv, app))
}

// Candidatas são as pastas, em ordem de preferência: a Local do cd2-harness
// (a mesma do cache) e depois a TEMP.
func Candidatas(env func(string) string, app string) []string {
	var c []string
	for _, v := range []string{"LOCALAPPDATA", "TEMP"} {
		if base := env(v); base != "" {
			c = append(c, base+`\cd2-harness\webview2\`+app)
		}
	}
	return c
}

// Escolher devolve a primeira candidata em que dá para gravar de fato (cria
// a pasta e um arquivo de teste), ou vazio se nenhuma serve.
func Escolher(candidatas []string) string {
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
