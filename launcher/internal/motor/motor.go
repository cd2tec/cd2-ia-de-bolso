// Package motor sobe o llamafile: tenta com GPU, cai para CPU se não subir e
// confere o /health até ele responder.
package motor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/proc"
)

// Prazo é quanto cada tentativa espera o motor ficar de pé: 240 s, como os
// scripts. Variável para o teste encurtar.
var Prazo = 240 * time.Second

// Intervalo é o tempo entre uma conferência do /health e a seguinte.
var Intervalo = time.Second

// AvisoGPU é o texto mostrado quando a GPU não serviu.
const AvisoGPU = "Aceleração por GPU indisponível, tentando só com CPU..."

var (
	// ErrMorreu indica que o motor saiu antes de ficar pronto.
	ErrMorreu = errors.New("o motor saiu antes de ficar pronto")
	// ErrPrazo indica que o motor não ficou pronto a tempo.
	ErrPrazo = errors.New("o motor não ficou pronto a tempo")
	// ErrNaoSubiu indica que nem com GPU nem com CPU o motor subiu.
	ErrNaoSubiu = errors.New("o motor não subiu")
)

// Config reúne o que o motor precisa; os caminhos já vêm prontos.
type Config struct {
	Exe, Modelo, Web, LogDir string
	Porta                    int
	Ctx                      string
	GOOS                     string
}

// Args monta os argumentos do llamafile. Sem aspas manuais: o exec cuida de
// caminho com espaço.
func Args(c Config, gpu bool) []string {
	a := []string{"-m", c.Modelo, "--server", "--host", "127.0.0.1", "--port", strconv.Itoa(c.Porta),
		"--jinja", "-c", c.Ctx, "--path", c.Web}
	if gpu {
		return append(a, "-ngl", "999")
	}
	return append(a, "--gpu", "disable", "-ngl", "0")
}

// iniciarFn é o Iniciar que o Subir usa; o teste troca para espiar os processos.
var iniciarFn = Iniciar

// Iniciar sobe o motor e zera o log. No Windows o stderr vai para
// server.err.log; fora dele, junto com o stdout em server.log.
func Iniciar(c Config, gpu bool) (*proc.Processo, error) {
	saida := filepath.Join(c.LogDir, "server.log")
	erros := ""
	if c.GOOS == "windows" {
		erros = filepath.Join(c.LogDir, "server.err.log")
	}
	return proc.Iniciar(c.Exe, Args(c, gpu), nil, saida, erros)
}

// Esperar confere GET url+"/health" até dar 200. Erro de conexão ou outro
// status significa "ainda não". Sai na hora se o processo morrer ou o ctx
// for cancelado.
func Esperar(ctx context.Context, url string, p *proc.Processo, prazo time.Duration, get func(string) (int, error)) error {
	limite := time.NewTimer(prazo)
	defer limite.Stop()
	tique := time.NewTicker(Intervalo)
	defer tique.Stop()
	for {
		select {
		case <-p.Morreu():
			return ErrMorreu
		default:
		}
		if st, err := get(url + "/health"); err == nil && st == 200 {
			return nil
		}
		select {
		case <-p.Morreu():
			return ErrMorreu
		case <-ctx.Done():
			return ctx.Err()
		case <-limite.C:
			return ErrPrazo
		case <-tique.C:
		}
	}
}

// Subir tenta com GPU; se não der, avisa, guarda o log da GPU como
// server-gpu.log e tenta só com CPU. Devolve o motor pronto, ou ErrNaoSubiu.
func Subir(ctx context.Context, c Config, get func(string) (int, error), aviso func(string)) (*proc.Processo, error) {
	url := "http://127.0.0.1:" + strconv.Itoa(c.Porta)
	// Falha ao iniciar (exe ausente, pasta de logs inválida) não é GPU
	// indisponível: não adianta avisar nem tentar de novo.
	var falhaIniciar error
	tentar := func(gpu bool) (*proc.Processo, error) {
		p, err := iniciarFn(c, gpu)
		if err != nil {
			falhaIniciar = err
			return nil, err
		}
		if err := Esperar(ctx, url, p, Prazo, get); err != nil {
			_ = p.Parar()
			return nil, err
		}
		return p, nil
	}
	p, err := tentar(true)
	if err == nil {
		return p, nil
	}
	if falhaIniciar != nil {
		return nil, fmt.Errorf("%w: %v", ErrNaoSubiu, falhaIniciar)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	aviso(AvisoGPU)
	// A tentativa seguinte zera o log; o da GPU explica a troca. O Parar já
	// esperou o processo sair, então no Windows o arquivo está solto.
	for _, par := range [][2]string{{"server.log", "server-gpu.log"}, {"server.err.log", "server-gpu.err.log"}} {
		novo := filepath.Join(c.LogDir, par[1])
		_ = os.Remove(novo)
		_ = os.Rename(filepath.Join(c.LogDir, par[0]), novo)
	}
	p, err = tentar(false)
	if err == nil {
		return p, nil
	}
	if falhaIniciar != nil {
		return nil, fmt.Errorf("%w: %v", ErrNaoSubiu, falhaIniciar)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return nil, fmt.Errorf("%w: %v", ErrNaoSubiu, err)
}
