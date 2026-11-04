package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/elevacao"
)

// nomeRegistro é o arquivo de log do instalador, na pasta cd2-harness:
// %LOCALAPPDATA%\cd2-harness no Windows, ~/Library/Caches/cd2-harness no Mac.
const nomeRegistro = "instalador.log"

// limiteRegistro: acima disso o arquivo recomeça do zero na próxima abertura.
const limiteRegistro = 2 << 20

// caminhoRegistro devolve onde fica o log, ou "" se não dá para saber.
func caminhoRegistro() string {
	pasta, err := pastaChavePadrao()
	if err != nil {
		return ""
	}
	return filepath.Join(pasta, nomeRegistro)
}

// abrirRegistro manda o log do pacote log também para o arquivo. No Windows
// o instalador é app de janela e o stderr some, então sem isso nada fica.
// Falhar aqui não impede o instalador de abrir.
func abrirRegistro(args []string) {
	caminho := caminhoRegistro()
	if caminho == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		return
	}
	if st, err := os.Stat(caminho); err == nil && st.Size() > limiteRegistro {
		_ = os.Remove(caminho)
	}
	f, err := os.OpenFile(caminho, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Printf("==== instalador %s, %s/%s, pid %d, admin=%v, args=%q",
		versao, goruntime.GOOS, goruntime.GOARCH, os.Getpid(), elevacao.Elevado(), args)
}

// detalheDoErro escreve a cadeia inteira do erro: a frase que a tela mostra
// esconde a causa (gravar.Erro.Error() devolve só a frase).
func detalheDoErro(err error) string {
	var partes []string
	for e := err; e != nil; e = errors.Unwrap(e) {
		partes = append(partes, fmt.Sprintf("%v (%T)", e, e))
		// errors.Join e afins: Unwrap() []error.
		if j, ok := e.(interface{ Unwrap() []error }); ok {
			for _, x := range j.Unwrap() {
				partes = append(partes, "  + "+detalheDoErro(x))
			}
			break
		}
	}
	return strings.Join(partes, " <- ")
}

// AbrirRegistro mostra o arquivo de log na pasta dele, para a pessoa mandar
// para o suporte.
func (a *App) AbrirRegistro() error {
	caminho := caminhoRegistro()
	if caminho == "" {
		return errors.New("Não sei onde fica o registro neste computador.")
	}
	var c *exec.Cmd
	switch goruntime.GOOS {
	case "windows":
		c = exec.Command("explorer.exe", "/select,", caminho)
	case "darwin":
		c = exec.Command("open", "-R", caminho)
	default:
		c = exec.Command("xdg-open", filepath.Dir(caminho))
	}
	if err := c.Start(); err != nil {
		log.Printf("abrir o registro: %v", err)
		return errors.New("Não consegui abrir a pasta. O registro fica em " + caminho)
	}
	go func() { _ = c.Wait() }()
	return nil
}
