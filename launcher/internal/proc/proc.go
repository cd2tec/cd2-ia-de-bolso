// Package proc sobe e derruba um processo filho sem janela, com a saída em
// arquivo. Serve ao motor e ao cd2-ctl.
package proc

import (
	"os"
	"os/exec"
	"sync"
	"time"
)

// EsperaTerm é quanto o Parar espera depois do SIGTERM antes do SIGKILL
// (fora do Windows). É variável para o teste encurtar.
var EsperaTerm = 10 * time.Second

// Processo é um filho em execução.
type Processo struct {
	cmd    *exec.Cmd
	morreu chan struct{}
	logs   []*os.File
	parar  sync.Mutex
}

// Iniciar zera e abre saida e erros e sobe exe. Se erros for vazio ou igual a
// saida, o stderr vai para o mesmo arquivo. env nil herda o ambiente; senão
// é o ambiente atual mais env.
func Iniciar(exe string, args, env []string, saida, erros string) (*Processo, error) {
	fs, err := os.Create(saida)
	if err != nil {
		return nil, err
	}
	logs := []*os.File{fs}
	fe := fs
	if erros != "" && erros != saida {
		if fe, err = os.Create(erros); err != nil {
			fs.Close()
			return nil, err
		}
		logs = append(logs, fe)
	}
	fechar := func() {
		for _, f := range logs {
			f.Close()
		}
	}
	cmd := exec.Command(exe, args...)
	cmd.Stdout, cmd.Stderr = fs, fe
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	ajustar(cmd)
	if err := cmd.Start(); err != nil {
		fechar()
		return nil, err
	}
	p := &Processo{cmd: cmd, morreu: make(chan struct{}), logs: logs}
	go func() {
		_ = cmd.Wait()
		fechar()
		close(p.morreu)
	}()
	return p, nil
}

// Pid devolve o número do processo.
func (p *Processo) Pid() int { return p.cmd.Process.Pid }

// Morreu fecha quando o processo sai.
func (p *Processo) Morreu() <-chan struct{} { return p.morreu }

// Parar encerra o processo e volta só depois que ele saiu. Pode ser chamado
// várias vezes.
func (p *Processo) Parar() error {
	p.parar.Lock()
	defer p.parar.Unlock()
	select {
	case <-p.morreu:
		return nil
	default:
	}
	err := encerrar(p)
	<-p.morreu
	return err
}
