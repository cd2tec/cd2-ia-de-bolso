// Package navegador abre a conversa como janela de aplicativo no Edge ou no
// Chrome (ou parecido), maximizada, e cai no navegador padrão se não houver.
package navegador

import (
	neturl "net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// URL é o endereço da conversa. O ?v=<versão> muda a cada versão para o
// navegador não reaproveitar o front antigo do cache; a chave vai no #, que
// não sai do navegador.
func URL(porta int, versao, chave string) string {
	return "http://127.0.0.1:" + strconv.Itoa(porta) + "/?v=" + versao + "#k=" + chave
}

// ComInstalar põe o instalar=1 na URL da conversa, que abre já com o modal
// de instalação. O parâmetro entra antes do #: o que vem depois dele é a
// chave e nunca chega ao servidor.
func ComInstalar(url string) string { return comParametro(url, "instalar=1") }

// ComAgenda põe o agenda=<consulta> na URL da conversa, que abre já no
// histórico daquele agendamento. Entra antes do #, como o ComInstalar.
func ComAgenda(url, consulta string) string {
	return comParametro(url, "agenda="+neturl.QueryEscape(consulta))
}

// comParametro põe par (já escapado) na query, antes do #.
func comParametro(url, par string) string {
	base, hash := url, ""
	if i := strings.IndexByte(url, '#'); i >= 0 {
		base, hash = url[:i], url[i:]
	}
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + par + hash
}

// appsMac, em ordem de preferência, em /Applications.
var appsMac = []string{"Google Chrome", "Microsoft Edge", "Brave Browser", "Chromium"}

// candidatosWindows: Edge primeiro (vem com o Windows), depois Chrome. O
// caminho é montado com "\" pelo goos passado, não pelo do sistema de teste;
// variável de ambiente vazia descarta o candidato.
func candidatosWindows(env func(string) string) []string {
	var cands []string
	for _, c := range []struct{ ambVar, resto string }{
		{"ProgramFiles(x86)", `\Microsoft\Edge\Application\msedge.exe`},
		{"ProgramFiles", `\Microsoft\Edge\Application\msedge.exe`},
		{"ProgramFiles", `\Google\Chrome\Application\chrome.exe`},
		{"ProgramFiles(x86)", `\Google\Chrome\Application\chrome.exe`},
		{"LOCALAPPDATA", `\Google\Chrome\Application\chrome.exe`},
	} {
		if base := env(c.ambVar); base != "" {
			cands = append(cands, base+c.resto)
		}
	}
	return cands
}

// Comando monta o comando que abre a url. tela é "L,A" (área útil, só no Mac)
// ou vazio. Sem navegador conhecido devolve o abridor padrão do sistema.
func Comando(goos string, url string, existe func(string) bool, env func(string) string, tela string) (string, []string) {
	switch goos {
	case "windows":
		for _, exe := range candidatosWindows(env) {
			if existe(exe) {
				return exe, []string{"--app=" + url, "--start-maximized"}
			}
		}
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		for _, app := range appsMac {
			if existe("/Applications/" + app + ".app") {
				// O Chrome do Mac ignora --start-maximized: o tamanho vem da área útil.
				args := []string{"-na", app, "--args", "--app=" + url, "--start-maximized"}
				if tela != "" {
					args = append(args, "--window-position=0,0", "--window-size="+tela)
				}
				return "open", args
			}
		}
		return "open", []string{url}
	}
	return "xdg-open", []string{url}
}

// Abrir dispara o navegador sem esperar o processo.
func Abrir(url string) error {
	existe := func(p string) bool { _, err := os.Stat(p); return err == nil }
	nome, args := Comando(runtime.GOOS, url, existe, os.Getenv, AreaUtil())
	c := exec.Command(nome, args...)
	if err := c.Start(); err != nil {
		return err
	}
	// Wait em segundo plano só colhe o processo (evita zumbi do open).
	go func() { _ = c.Wait() }()
	return nil
}
