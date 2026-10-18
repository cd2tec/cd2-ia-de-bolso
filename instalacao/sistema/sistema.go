// Package sistema faz a parte da instalação que depende do sistema
// operacional: atalhos, início automático com o computador, o registro de
// "programas instalados" e o comando que apaga a pasta depois que o lançador
// sai. Nada aqui pede direitos de administrador.
package sistema

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Comando é o que o lançador dispara antes de sair para apagar a pasta da
// instalação. É só dado: quem roda é o lançador.
//
// No Windows, Args é {"/c", linha}, e a linha tem aspas que o cmd.exe precisa
// receber como estão. O escape padrão do Go (\") estraga essas aspas, então o
// lançador deve montar a linha de comando crua (SysProcAttr.CmdLine) como
// `cmd.exe /c ` + linha.
type Comando struct {
	Exe  string
	Args []string
}

// Sistema é o que muda entre Windows e Mac na instalação.
type Sistema interface {
	// Atalhos cria o atalho do Menu Iniciar (Windows) e, se areaDeTrabalho,
	// o da área de trabalho. Sem areaDeTrabalho, tira o que houver lá.
	Atalhos(lancador string, areaDeTrabalho bool) error
	// LigarInicio liga ou desliga a abertura junto com o computador.
	LigarInicio(lancador string, ligar bool) error
	InicioLigado() bool
	// Registrar põe a IA de Bolso na lista de programas instalados.
	Registrar(lancador, versao string) error
	// Remover desfaz Atalhos e Registrar. Não reclama do que já não existe.
	Remover() error
	// PlanoRemocao devolve o comando que espera o lançador sair e apaga destino.
	PlanoRemocao(destino string, pidLancador int) Comando
}

const (
	nomeLancador = "IA de Bolso"
	pastaWindows = "CD2 IA de Bolso"
	pastaMac     = "IA de Bolso"
)

// ErrSistema indica um sistema em que a instalação não existe.
var ErrSistema = errors.New("A instalação só funciona no Windows e no Mac.")

// Efeitos de sistema em variáveis, para os testes trocarem.
var (
	rodar = func(exe string, args ...string) error {
		ctx, cancelar := context.WithTimeout(context.Background(), limiteComando)
		defer cancelar()
		c := exec.CommandContext(ctx, exe, args...)
		esconderJanela(c)
		err := c.Run()
		if ctx.Err() != nil {
			return errors.New("o sistema demorou demais para responder")
		}
		return err
	}
	// limiteComando corta um comando que trava, como o osascript esperando a
	// pessoa responder ao pedido de permissão para controlar o Finder.
	limiteComando = time.Minute
	pastaPessoal  = func() string {
		h, _ := os.UserHomeDir()
		return h
	}
	ambiente = os.Getenv
)

// Destino devolve a pasta da instalação. O caminho usa o separador do goos
// informado, não o da máquina em que o código roda, para os testes rodarem
// em qualquer lugar.
func Destino(goos string, env func(string) string, home string) string {
	switch goos {
	case "windows":
		return env("LOCALAPPDATA") + `\Programs\` + pastaWindows
	case "darwin":
		return home + "/Applications/" + pastaMac
	}
	return home + "/.local/share/" + pastaMac
}

// Lancador devolve o executável do lançador dentro da pasta da instalação.
func Lancador(destino, goos string) string {
	switch goos {
	case "windows":
		return destino + `\` + nomeLancador + ".exe"
	case "darwin":
		return destino + "/" + nomeLancador + ".app/Contents/MacOS/" + nomeLancador
	}
	return destino + "/" + nomeLancador
}

// Do devolve o Sistema do goos informado.
func Do(goos string) Sistema {
	switch goos {
	case "windows":
		return windows{}
	case "darwin":
		return mac{}
	}
	return nenhum{}
}

// pai devolve o caminho sem o último elemento, pelo separador informado.
func pai(caminho, sep string) string {
	if i := strings.LastIndex(caminho, sep); i >= 0 {
		return caminho[:i]
	}
	return caminho
}

// removerSeHouver apaga um arquivo e trata "não existe" como sucesso.
func removerSeHouver(arq string) error {
	if err := os.Remove(arq); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
