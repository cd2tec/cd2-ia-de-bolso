package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/origem"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/sistema"

	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/cache"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/sessao"
)

// A IA instalada no dia a dia: o "Acordar quando o computador ligar" do
// ícone e o "Desinstalar...", que pergunta na janelinha antes.

// Fases da tela que não são da sessão: a pergunta, a despedida depois de
// disparar a remoção.
const (
	faseDesinstalar  = "Desinstalar"
	faseDesinstalada = "Desinstalada"
)

// Textos curtos dos problemas. Os detalhes vão para o lancador.log.
const (
	textoNaoMudouInicio      = "Não consegui mudar o acordar com o computador. Tente de novo."
	textoNaoDesinstalou      = "Não consegui me desinstalar. Os registros têm o motivo."
	textoNaoDesligouParaSair = "Não consegui desligar tudo, então não me desinstalei. Reinicie o computador e tente de novo."
)

// Atrás de variáveis para o teste não mexer na máquina de verdade.
var (
	sistemaAqui      = func() sistema.Sistema { return sistema.Do(runtime.GOOS) }
	desinstalarAgora = instalacao.Desinstalar
	dispararRemocao  = iniciarRemocao
	anotarFalha      = func(versao string, err error) {
		home, _ := os.UserHomeDir()
		if e := registrarFalha(cache.PastaLogs(runtime.GOOS, os.Getenv, home), versao, err); e != nil {
			log.Printf("não consegui registrar a falha: %v", e)
		}
	}
	// esperaDesinstalada é quanto o "Pronto, fui desinstalada" fica na tela.
	esperaDesinstalada = 2 * time.Second
)

// instalada diz se a IA roda da cópia instalada: só aí o menu tem o
// acordar com o computador e o desinstalar.
func (a *App) instalada() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.origem == origem.Instalada
}

// lerInicio guarda se o acordar com o computador está ligado, para a
// marcação do menu não ler o sistema a cada estado.
func (a *App) lerInicio() {
	ligado := sistemaAqui().InicioLigado()
	a.mu.Lock()
	a.inicioLigado = ligado
	a.mu.Unlock()
}

// alternarInicio é o clique no "Acordar quando o computador ligar": liga ou
// desliga na hora. Se o sistema recusar, a marcação fica como o sistema
// está (a de antes) e a janela mostra o aviso.
func (a *App) alternarInicio() {
	s := sistemaAqui()
	err := s.LigarInicio(caminhoInstalada(), !s.InicioLigado())
	ligado := s.InicioLigado()
	a.mu.Lock()
	tinhaAviso := a.avisoInicio != ""
	a.inicioLigado = ligado
	a.avisoInicio = ""
	if err != nil {
		a.avisoInicio = textoNaoMudouInicio
	}
	a.mu.Unlock()
	a.atualizarBandeja(a.s.Atual())
	if err != nil {
		log.Printf("não consegui mudar o acordar com o computador: %v", err)
		a.emitirAgora()
		a.mostrar()
		return
	}
	if tinhaAviso {
		a.emitirAgora()
	}
}

// telaEspecial é a tela que não sai da sessão, na ordem em que vale: a
// desinstalação que falhou, a despedida, a pergunta e a trava.
func (a *App) telaEspecial() (estadoTela, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case a.ficou:
		return estadoTela{Fase: "Erro", Erro: a.erroFicou}, true
	case a.desinstalada:
		return estadoTela{Fase: faseDesinstalada}, true
	case a.perguntando:
		return estadoTela{Fase: faseDesinstalar}, true
	case a.trancada:
		return estadoTela{Fase: faseTrancada}, true
	}
	return estadoTela{}, false
}

// naPergunta diz se a tela está perguntando se pode desinstalar: os
// estados da sessão esperam a resposta.
func (a *App) naPergunta() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.perguntando
}

// emitirAgora manda para a tela o estado de agora.
func (a *App) emitirAgora() {
	if ctx := a.contexto(); ctx != nil {
		wruntime.EventsEmit(ctx, "estado", a.Estado())
	}
}

// PedirDesinstalar é o "Desinstalar..." do ícone e o --desinstalar: mostra
// a pergunta. Indo dormir (ou já desinstalando) só traz a janela.
func (a *App) PedirDesinstalar() {
	f := a.s.Atual().Fase
	a.mu.Lock()
	pode := !a.desinstalando && !a.ficou && f != sessao.Dormindo && f != sessao.Dormiu
	if pode {
		a.perguntando = true
	}
	a.mu.Unlock()
	if pode {
		a.emitirAgora()
	}
	a.mostrar()
}

// MelhorNao volta da pergunta para a tela de antes. Se a IA ainda não
// acordou (o --desinstalar), acorda agora; trancada, volta para a trava.
func (a *App) MelhorNao() {
	a.mu.Lock()
	a.perguntando = false
	tranca := a.trancada
	a.mu.Unlock()
	a.emitirAgora()
	if !tranca {
		a.acordar() // o Rodar não sobe duas vezes
	}
}

// Desinstalar é o botão da pergunta. A IA dorme primeiro; o resto vem no
// Dormiu (terminarDesinstalar), como a troca para a instalada.
func (a *App) Desinstalar() {
	a.mu.Lock()
	if a.desinstalando {
		a.mu.Unlock()
		return
	}
	a.desinstalando = true
	a.perguntando = false
	a.mu.Unlock()
	a.mostrar()
	a.Dormir()
}

// estaDesinstalando diz se o Dormiu que chegar é o da desinstalação.
func (a *App) estaDesinstalando() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.desinstalando
}

// terminarDesinstalar roda no Dormiu: desfaz o início, os atalhos e o
// registro, dispara a remoção da pasta (que espera este processo sair) e
// sai. Se algo recusar, o app fica, com a IA dormindo e o erro na tela.
func (a *App) terminarDesinstalar(dormiu string) {
	if dormiu != "" {
		a.ficarViva(textoNaoDesligouParaSair, errors.New(dormiu))
		return
	}
	c, err := desinstalarAgora(pastaCache(), pidAtual())
	if err == nil && c.Exe == "" {
		err = errors.New("a remoção da pasta veio vazia")
	}
	if err == nil {
		err = dispararRemocao(c)
	}
	if err != nil {
		a.ficarViva(textoNaoDesinstalou, err)
		return
	}
	a.mu.Lock()
	a.desinstalada = true
	a.mu.Unlock()
	a.emitirAgora()
	go func() {
		time.Sleep(esperaDesinstalada)
		a.sair()
	}()
}

// ficarViva é a desinstalação que não aconteceu: o erro fica na tela, o
// menu só mostra ou fecha e fechar a janela sai.
func (a *App) ficarViva(texto string, err error) {
	log.Printf("não desinstalei: %v", err)
	anotarFalha(a.versao, fmt.Errorf("desinstalar: %w", err))
	a.mu.Lock()
	a.ficou = true
	a.erroFicou = texto
	a.mu.Unlock()
	a.emitirAgora()
	a.atualizarBandeja(a.s.Atual())
	a.mostrar()
}

// comandoRemocao monta a remoção da pasta: solta e escondida, com o
// ambiente de agora mais o do comando, e rodando de fora da pasta que vai
// ser apagada.
func comandoRemocao(c sistema.Comando) *exec.Cmd {
	cmd := exec.Command(c.Exe, c.Args...)
	cmd.Env = append(os.Environ(), c.Env...)
	cmd.Dir = os.TempDir()
	soltarEscondido(cmd)
	return cmd
}

// iniciarRemocao roda a remoção sem esperar por ela: ela é quem espera
// este processo sair.
func iniciarRemocao(c sistema.Comando) error {
	cmd := comandoRemocao(c)
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	return nil
}
