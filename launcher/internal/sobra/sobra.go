// Package sobra encerra o llamafile que ficou de uma abertura anterior. No
// Windows ele segura o server.log aberto e a abertura seguinte não consegue
// zerar o log. Só mata o llamafile deste pen drive, nunca outro. É a mesma
// regra do Stop-LeftoverLlamafile de core/start-windows.ps1. Fora do Windows
// não faz nada: no Mac o motor roda de uma cópia no cache e não trava arquivo.
package sobra

import (
	"errors"
	"os"
	"strings"
	"time"
)

// ErrAindaFechando indica que o log continuou preso depois de encerrar a IA
// anterior.
var ErrAindaFechando = errors.New("a IA anterior ainda está fechando")

// Tentativas e Pausa dão o tempo que o log tem para soltar: 25 × 200 ms,
// como o script. Variáveis para o teste encurtar.
var (
	Tentativas = 25
	Pausa      = 200 * time.Millisecond
)

// Processo é um llamafile.exe rodando, com o caminho do executável (vazio se
// o Windows não deixou ler).
type Processo struct {
	PID uint32
	Exe string
}

// listarFn, matarFn e abrirFn são os efeitos; o teste troca por falsos.
var (
	listarFn = listar
	matarFn  = matar
	abrirFn  = func(caminho string) error {
		f, err := os.OpenFile(caminho, os.O_WRONLY, 0)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		return f.Close()
	}
)

// DoPenDrive diz se exe é o mesmo executável que alvo, com as regras de
// caminho do Windows: barra para qualquer lado, sem diferença de maiúscula
// e sem o prefixo \\?\. Caminho vazio nunca casa.
func DoPenDrive(alvo, exe string) bool {
	a, e := normalizar(alvo), normalizar(exe)
	return a != "" && e != "" && strings.EqualFold(a, e)
}

func normalizar(p string) string {
	p = strings.ReplaceAll(strings.TrimSpace(p), "/", `\`)
	p = strings.TrimPrefix(p, `\\?\`)
	return strings.TrimRight(p, `\`)
}

// Encerrar mata o llamafile que roda de alvo (o do pen drive) e, se matou
// algum, espera os logs soltarem. Devolve ErrAindaFechando se algum log
// continuar preso.
func Encerrar(alvo string, logs []string) error {
	procs, err := listarFn()
	if err != nil {
		return nil // sem lista, segue como antes: na pior hipótese o Iniciar falha
	}
	matou := false
	for _, p := range procs {
		if DoPenDrive(alvo, p.Exe) {
			_ = matarFn(p.PID)
			matou = true
		}
	}
	if !matou {
		return nil
	}
	for _, l := range logs {
		if !esperarSolto(l) {
			return ErrAindaFechando
		}
	}
	return nil
}

func esperarSolto(caminho string) bool {
	for i := 0; i < Tentativas; i++ {
		if abrirFn(caminho) == nil {
			return true
		}
		time.Sleep(Pausa)
	}
	return false
}
