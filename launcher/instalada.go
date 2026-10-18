package main

import (
	"log"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/origem"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/sistema"
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

// segundaCopia é outro lançador aberto com este rodando. Com --assumir e a
// instalada no disco, este (o do pen drive) dorme e passa a vez; em qualquer
// outro caso só traz a janela.
func (a *App) segundaCopia(args []string) {
	if !lerArgs(args).Assumir || !a.pendrive() {
		a.chamar()
		return
	}
	exe := caminhoInstalada()
	if !existeArquivo(exe) {
		a.chamar()
		return
	}
	a.mu.Lock()
	a.trocarPara = exe
	a.mu.Unlock()
	a.mostrar()
	a.Dormir()
}

// trocar roda a instalada depois do Dormiu, se a troca foi pedida. Se algo
// não desligou, não troca: o aviso de reiniciar fica na tela. Devolve se
// rodou.
func (a *App) trocar(dormiu string) bool {
	a.mu.Lock()
	exe := a.trocarPara
	a.mu.Unlock()
	if exe == "" || dormiu != "" {
		return false
	}
	if err := iniciarDestacado(exe, "--esperar", strconv.Itoa(pidAtual())); err != nil {
		log.Printf("não consegui abrir a IA instalada (%s): %v", exe, err)
		return false
	}
	return true
}

// AbrirInstalar é o convite: abre a conversa já com o modal de instalação.
func (a *App) AbrirInstalar() error { return a.s.AbrirInstalar() }
