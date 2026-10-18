package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/origem"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/sistema"

	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/sessao"
)

// A IA rodando do pen drive e a instalada no computador: o convite para
// instalar, a troca para a instalada (--assumir) e o início escondido com o
// computador (--inicio).

// Args são os argumentos que o lançador entende. O resto (o -psn_... que o
// Mac às vezes passa, por exemplo) é ignorado.
type Args struct {
	Inicio      bool // acordou com o computador: só o ícone, sem janela nem conversa
	Assumir     bool // o cd2-ctl abriu a instalada: a do pen drive dorme e passa a vez
	Desinstalar bool
	// Esperar é o pid do lançador do pen drive que passou a vez: a instalada
	// espera ele sair, porque os dois disputam a mesma trava de cópia única.
	Esperar int
}

// lerArgs lê os argumentos, sem o nome do programa.
func lerArgs(args []string) Args {
	var a Args
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--inicio":
			a.Inicio = true
		case "--assumir":
			a.Assumir = true
		case "--desinstalar":
			a.Desinstalar = true
		case "--esperar":
			if i+1 < len(args) {
				if pid, err := strconv.Atoi(args[i+1]); err == nil && pid > 0 {
					a.Esperar = pid
					i++
				}
			}
		}
	}
	return a
}

// prazoEsperar é quanto a instalada espera o lançador do pen drive sair.
// Passado o prazo ela segue: na pior das hipóteses vira uma segunda cópia
// que só traz a janela da outra.
const prazoEsperar = 30 * time.Second

// esperarProcesso espera o pid sair, perguntando a cada passo, até o prazo.
func esperarProcesso(pid int, prazo, passo time.Duration, vivo func(int) bool) {
	fim := time.Now().Add(prazo)
	for vivo(pid) && time.Now().Before(fim) {
		time.Sleep(passo)
	}
}

// Atrás de variáveis para o teste não rodar nada de verdade.
var (
	caminhoInstalada = func() string {
		home, _ := os.UserHomeDir()
		return sistema.Lancador(sistema.Destino(runtime.GOOS, os.Getenv, home), runtime.GOOS)
	}
	existeArquivo = func(p string) bool {
		st, err := os.Stat(p)
		return err == nil && !st.IsDir()
	}
	iniciarDestacado = iniciarSolto
	pidAtual         = os.Getpid
)

// pendrive diz se a IA roda do pen drive: só aí aparece o convite.
func (a *App) pendrive() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.origem == origem.Pendrive
}

// textoNaoAbriuInstalada substitui o "Boa noite! Pode tirar o pen drive."
// quando a IA do pen drive dormiu para passar a vez e a instalada não abriu:
// sem ele a pessoa ficaria sem IA nenhuma achando que deu certo.
const textoNaoAbriuInstalada = "Não consegui abrir a IA instalada. Abra pelo ícone nos programas."

// segundaCopia é outro lançador aberto com este rodando. Com --assumir e a
// instalada no disco, este (o do pen drive) dorme e passa a vez; em qualquer
// outro caso só traz a janela.
func (a *App) segundaCopia(args []string) {
	if !lerArgs(args).Assumir {
		a.chamar()
		return
	}
	if !a.pendrive() {
		log.Printf("--assumir ignorado: esta IA não roda do pen drive")
		a.chamar()
		return
	}
	if a.s.Atual().Fase == sessao.Dormiu {
		log.Printf("--assumir ignorado: a IA do pen drive já dormiu")
		return
	}
	exe := caminhoInstalada()
	if !existeArquivo(exe) {
		log.Printf("--assumir ignorado: não achei a IA instalada em %s", exe)
		a.chamar()
		return
	}
	a.mu.Lock()
	a.trocarPara = exe
	a.mu.Unlock()
	a.mostrar()
	a.Dormir()
}

// Resultado da troca no Dormiu.
type troca int

const (
	semTroca troca = iota // não foi pedida, ou algo não desligou
	trocou                // a instalada foi aberta
	naoAbriu              // pedida, tudo desligou, mas a instalada não abriu
)

// trocar roda a instalada depois do Dormiu, se a troca foi pedida. Se algo
// não desligou, não troca: o aviso de reiniciar fica na tela.
func (a *App) trocar(dormiu string) troca {
	a.mu.Lock()
	exe := a.trocarPara
	a.mu.Unlock()
	if exe == "" || dormiu != "" {
		return semTroca
	}
	if err := iniciarDestacado(exe, "--esperar", strconv.Itoa(pidAtual())); err != nil {
		log.Printf("não consegui abrir a IA instalada (%s): %v", exe, err)
		a.mu.Lock()
		a.naoAbriu = true
		a.mu.Unlock()
		return naoAbriu
	}
	return trocou
}

// comDormiu põe no Dormiu a falha de abrir a instalada, para a tela e a
// janela tratarem como o Dormiu com erro (aviso visível, sem piada, 15 s).
func (a *App) comDormiu(e sessao.Estado) sessao.Estado {
	a.mu.Lock()
	falhou := a.naoAbriu
	a.mu.Unlock()
	if falhou && e.Fase == sessao.Dormiu && e.Erro == "" {
		e.Erro = textoNaoAbriuInstalada
	}
	return e
}

// comandoSolto monta o comando da instalada. Roda de dentro da pasta dela:
// herdar a pasta atual (o pen drive) seguraria o pen drive e impediria
// tirá-lo com segurança.
func comandoSolto(exe string, args ...string) *exec.Cmd {
	c := exec.Command(exe, args...)
	c.Dir = filepath.Dir(exe)
	soltar(c)
	return c
}

// iniciarSolto roda o programa sem esperar por ele.
func iniciarSolto(exe string, args ...string) error {
	c := comandoSolto(exe, args...)
	if err := c.Start(); err != nil {
		return err
	}
	go func() { _ = c.Wait() }() // só colhe o processo
	return nil
}

// AbrirInstalar é o convite: abre a conversa já com o modal de instalação.
func (a *App) AbrirInstalar() error { return a.s.AbrirInstalar() }
