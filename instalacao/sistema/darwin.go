package sistema

import (
	"bytes"
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

// as escreve s como texto do AppleScript, entre aspas duplas.
func as(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// Atalhos põe um apelido na área de trabalho pelo Finder. No Mac não há Menu
// Iniciar: o .app já aparece no Spotlight e no Launchpad.
func (mac) Atalhos(lancador string, areaDeTrabalho bool) error {
	if err := removerSeHouver(arquivoApelido()); err != nil {
		return fmt.Errorf("não consegui trocar o atalho da área de trabalho: %w", err)
	}
	if !areaDeTrabalho {
		return nil
	}
	script := `tell application "Finder" to make alias file to POSIX file ` + as(appDoLancador(lancador)) +
		` at POSIX file ` + as(filepath.Join(pastaPessoal(), "Desktop")) +
		` with properties {name:` + as(nomeApelido) + `}`
	if err := rodar("/usr/bin/osascript", "-e", script); err != nil {
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
	_, err := os.Stat(arquivoAgente())
	return err == nil
}

// Registrar não faz nada no Mac: a pasta Aplicativos já mostra o .app.
func (mac) Registrar(string, string) error { return nil }

// Remover tira o apelido da área de trabalho. Não há registro a desfazer.
func (mac) Remover() error {
	if err := removerSeHouver(arquivoApelido()); err != nil {
		return fmt.Errorf("não consegui tirar o atalho da área de trabalho: %w", err)
	}
	return nil
}

// sh escreve s como texto do shell, entre aspas simples.
func sh(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// PlanoRemocao espera até 30 s o lançador sair e apaga a pasta.
func (mac) PlanoRemocao(destino string, pid int) Comando {
	linha := fmt.Sprintf(`cd /; i=0; while kill -0 %d 2>/dev/null && [ $i -lt 30 ]; do sleep 1; i=$((i+1)); done; rm -rf %s`,
		pid, sh(destino))
	return Comando{Exe: "/bin/sh", Args: []string{"-c", linha}}
}
