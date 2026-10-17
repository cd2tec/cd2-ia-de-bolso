// Package ctl sobe o cd2-ctl, o controle do banco que roda ao lado do motor.
package ctl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/proc"
)

// Intervalo é o tempo entre uma conferência do /status e a seguinte (1 s,
// como os scripts). Variável para o teste encurtar.
var Intervalo = time.Second

// tentativas é quantas vezes o /status é conferido antes de desistir.
const tentativas = 10

// ErrNaoSubiu indica que o cd2-ctl não ficou no ar.
var ErrNaoSubiu = errors.New("o controle do banco não subiu")

// Caminho devolve o executável do cd2-ctl dentro do pen drive. O separador
// segue o sistema de destino, não o de quem roda (a raiz de drive "E:\"
// não passa bem por filepath.Join fora do Windows).
func Caminho(raiz, goos, goarch string) string {
	sep, nome, plat := "/", "cd2-ctl", "darwin-amd64"
	if goos == "windows" {
		sep, nome, plat = `\`, "cd2-ctl.exe", "windows-amd64"
	} else if goarch == "arm64" {
		plat = "darwin-arm64"
	}
	return strings.TrimRight(raiz, `\/`) + sep + strings.Join([]string{"bin", "ctl", plat, nome}, sep)
}

// Args monta os argumentos do cd2-ctl. A chave não entra aqui: vai só pelo
// ambiente do filho.
func Args(raiz, cache string, portaHarness, pidPai int) []string {
	return []string{"--root", raiz, "--cache", cache,
		"--harness-port", strconv.Itoa(portaHarness), "--parent-pid", strconv.Itoa(pidPai)}
}

// Subir inicia o cd2-ctl com CD2_CTL_KEY só no ambiente dele e confere
// GET /status até ele responder; 401 (sem chave) significa no ar. No
// Windows o stderr vai para ctl.err.log; fora dele, junto com o stdout em
// ctl.log. Se não subir, para o processo e devolve ErrNaoSubiu com as
// últimas 5 linhas do log.
func Subir(ctx context.Context, exe string, args []string, chave string, portaCtl int, logDir, goos string, get func(string) (int, error)) (*proc.Processo, error) {
	saida := filepath.Join(logDir, "ctl.log")
	erros, logErro := "", saida
	if goos == "windows" {
		erros = filepath.Join(logDir, "ctl.err.log")
		logErro = erros
	}
	p, err := proc.Iniciar(exe, args, []string{"CD2_CTL_KEY=" + chave}, saida, erros)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNaoSubiu, err)
	}
	falhou := func(motivo error) (*proc.Processo, error) {
		_ = p.Parar()
		msg := motivo.Error()
		if fim := fimDoLog(logErro, 5); fim != "" {
			msg += ":\n" + fim
		}
		return nil, fmt.Errorf("%w: %s", ErrNaoSubiu, msg)
	}
	url := "http://127.0.0.1:" + strconv.Itoa(portaCtl) + "/status"
	for i := 0; i < tentativas; i++ {
		select {
		case <-p.Morreu():
			return falhou(errors.New("o processo saiu"))
		default:
		}
		if st, err := get(url); err == nil && st == 401 {
			return p, nil
		}
		select {
		case <-p.Morreu():
			return falhou(errors.New("o processo saiu"))
		case <-ctx.Done():
			_ = p.Parar()
			return nil, ctx.Err()
		case <-time.After(Intervalo):
		}
	}
	return falhou(errors.New("não respondeu a tempo"))
}

// fimDoLog devolve as últimas n linhas do arquivo, ou "" se não der para ler.
func fimDoLog(arq string, n int) string {
	b, err := os.ReadFile(arq)
	if err != nil {
		return ""
	}
	linhas := strings.Split(strings.TrimRight(string(b), "\r\n"), "\n")
	if len(linhas) > n {
		linhas = linhas[len(linhas)-n:]
	}
	return strings.TrimSpace(strings.Join(linhas, "\n"))
}
