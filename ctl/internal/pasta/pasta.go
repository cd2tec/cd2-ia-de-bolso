// Package pasta abre o seletor de pastas nativo do sistema (Finder no Mac, o do Windows, zenity
// ou kdialog no Linux) para a tela escolher a pasta dos motores de arquivo (DBF e Planilhas Excel).
// O navegador não entrega o caminho completo de uma pasta escolhida nele, por isso quem abre o
// seletor é o cd2-ctl, que roda na máquina da pessoa.
package pasta

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// ErrSemSeletor: o sistema não tem seletor de pastas que o cd2-ctl saiba abrir (Linux sem
// zenity nem kdialog).
var ErrSemSeletor = errors.New("Este computador não tem um seletor de pastas que a IA de Bolso consiga abrir. Cole o caminho da pasta no campo")

// comando é o seletor de um sistema: programa, argumentos, variáveis extras e como reconhecer
// o Cancelar.
type comando struct {
	nome     string
	args     []string
	env      []string
	cancelou func(code int, stderr string) bool
}

// lookPath é o exec.LookPath, trocado nos testes.
var lookPath = exec.LookPath

// Escolher abre o seletor começando em inicial (ou na pasta pessoal, se inicial não for uma pasta
// que existe). Devolve o caminho escolhido e ok; ok falso e erro nil quando a pessoa cancelou.
func Escolher(ctx context.Context, inicial string) (string, bool, error) {
	c, err := seletor(pastaInicial(inicial))
	if err != nil {
		return "", false, err
	}
	cmd := exec.CommandContext(ctx, c.nome, c.args...)
	cmd.Env = append(os.Environ(), c.env...)
	esconder(cmd)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	runErr := cmd.Run()
	return interpretar(out.String(), errb.String(), runErr, c.cancelou)
}

// interpretar traduz a saída do seletor: o caminho na saída padrão, o Cancelar pelo código de
// saída (ou pela mensagem, no osascript), o resto como erro.
func interpretar(stdout, stderr string, runErr error, cancelou func(int, string) bool) (string, bool, error) {
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) && cancelou(ee.ExitCode(), stderr) {
			return "", false, nil
		}
		if msg := strings.TrimSpace(stderr); msg != "" {
			return "", false, errors.New("O seletor de pastas falhou: " + msg)
		}
		return "", false, errors.New("O seletor de pastas falhou: " + runErr.Error())
	}
	p := limpar(stdout)
	if p == "" {
		return "", false, nil
	}
	return p, true, nil
}

// limpar tira a quebra de linha do fim e a barra final que o Finder e o zenity põem ("/x/" vira
// "/x"), sem mexer na raiz ("/" e "C:\").
func limpar(s string) string {
	s = strings.TrimRight(s, "\r\n")
	for len(s) > 1 && (strings.HasSuffix(s, "/") || strings.HasSuffix(s, `\`)) && !strings.HasSuffix(s, `:\`) {
		s = s[:len(s)-1]
	}
	return s
}

// pastaInicial devolve inicial se for uma pasta que existe (sem as aspas do "Copiar como caminho"
// do Windows), senão a pasta pessoal, senão "".
func pastaInicial(inicial string) string {
	p := strings.TrimSpace(inicial)
	if len(p) >= 2 && strings.HasPrefix(p, `"`) && strings.HasSuffix(p, `"`) {
		p = strings.TrimSpace(p[1 : len(p)-1])
	}
	if p != "" {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
	}
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return ""
}

func porCodigo(n int) func(int, string) bool {
	return func(code int, _ string) bool { return code == n }
}
