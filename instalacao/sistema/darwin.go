package sistema

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"howett.net/plist"
)

// Sem build tag: só arquivos e textos, e os testes rodam em qualquer máquina.

const (
	rotuloAgente = "br.com.cd2.iadebolso"
	nomeApelido  = "IA de Bolso"
)

type mac struct{}

func arquivoAgente() string {
	return filepath.Join(pastaPessoal(), "Library", "LaunchAgents", rotuloAgente+".plist")
}

func arquivoApelido() string {
	return filepath.Join(pastaPessoal(), "Desktop", nomeApelido)
}

// appDoLancador sobe de .../X.app/Contents/MacOS/X para .../X.app.
func appDoLancador(lancador string) string {
	return pai(pai(pai(lancador, "/"), "/"), "/")
}

// removerApelido tira o atalho da área de trabalho, mas só se for um link
// simbólico: um arquivo da pessoa com o mesmo nome fica onde está.
func removerApelido() error {
	arq := arquivoApelido()
	fi, err := os.Lstat(arq)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	return removerSeHouver(arq)
}

// Atalhos põe na área de trabalho um link simbólico para o .app. Não usa o
// Finder, que pediria permissão para ser controlado. No Mac não há Menu
// Iniciar: o .app já aparece no Spotlight e no Launchpad.
func (mac) Atalhos(lancador string, areaDeTrabalho bool) error {
	if pastaPessoal() == "" {
		return ErrSemPastaPessoal
	}
	if err := removerApelido(); err != nil {
		return fmt.Errorf("não consegui trocar o atalho da área de trabalho: %w", err)
	}
	if !areaDeTrabalho {
		return nil
	}
	if err := os.Symlink(appDoLancador(lancador), arquivoApelido()); err != nil {
		return fmt.Errorf("não consegui criar o atalho na área de trabalho: %w", err)
	}
	return nil
}

// agente é o LaunchAgent que abre a IA de Bolso quando a pessoa entra no Mac.
type agente struct {
	Label            string
	ProgramArguments []string
	RunAtLoad        bool
}

// LigarInicio grava ou apaga o LaunchAgent. Não chama o launchctl: o
// RunAtLoad basta para a próxima vez que a pessoa entrar no Mac.
func (mac) LigarInicio(lancador string, ligar bool) error {
	if pastaPessoal() == "" {
		return ErrSemPastaPessoal
	}
	arq := arquivoAgente()
	if !ligar {
		if err := removerSeHouver(arq); err != nil {
			return fmt.Errorf("não consegui desligar o início com o computador: %w", err)
		}
		return nil
	}
	var b bytes.Buffer
	enc := plist.NewEncoder(&b)
	enc.Indent("\t")
	if err := enc.Encode(agente{Label: rotuloAgente, ProgramArguments: []string{lancador, "--inicio"}, RunAtLoad: true}); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(arq), 0o755); err != nil {
		return fmt.Errorf("não consegui ligar o início com o computador: %w", err)
	}
	if err := os.WriteFile(arq, b.Bytes(), 0o644); err != nil {
		return fmt.Errorf("não consegui ligar o início com o computador: %w", err)
	}
	return nil
}

func (mac) InicioLigado() bool {
	if pastaPessoal() == "" {
		return false
	}
	_, err := os.Stat(arquivoAgente())
	return err == nil
}

// Registrar não faz nada no Mac: a pasta Aplicativos já mostra o .app.
func (mac) Registrar(string, string) error { return nil }

// Remover tira o atalho da área de trabalho. Não há registro a desfazer.
func (mac) Remover() error {
	if pastaPessoal() == "" {
		return ErrSemPastaPessoal
	}
	if err := removerApelido(); err != nil {
		return fmt.Errorf("não consegui tirar o atalho da área de trabalho: %w", err)
	}
	return nil
}

// PlanoRemocao espera até 30 s o lançador sair e apaga a pasta. O caminho
// vai em CD2_DESTINO, não no texto do script.
func (mac) PlanoRemocao(destino string, pid int) Comando {
	if !strings.HasPrefix(destino, "/") || base(destino, "/") != pastaMac || pid <= 0 {
		return Comando{}
	}
	linha := fmt.Sprintf(`cd /; i=0; while kill -0 %d 2>/dev/null && [ $i -lt 30 ]; do sleep 1; i=$((i+1)); done; rm -rf "$CD2_DESTINO"`, pid)
	return Comando{Exe: "/bin/sh", Args: []string{"-c", linha}, Env: []string{"CD2_DESTINO=" + destino}}
}
