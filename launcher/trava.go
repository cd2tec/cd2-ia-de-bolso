package main

import (
	"os"
	"runtime"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/chave"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/codigo"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/guarda"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/origem"

	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/cache"
)

// A trava da cópia sem chave: instalada no disco e sem uma chave que valha
// para este computador, a IA não sobe. A janelinha mostra a fase Trancada e
// deixa colar a chave.

// faseTrancada é a fase da tela. Não é uma fase da sessão: a sessão nem
// começou.
const faseTrancada = "Trancada"

// Respostas do ColarChave. A da chave incompleta é a mesma da conversa.
const (
	textoChaveIncompleta = "Esta chave não está completa. Confira se copiou ela inteira."
	textoChaveDeOutro    = "Esta chave não é deste computador."
	textoSemCodigo       = "Não consegui ler o código deste computador."
	textoNaoGravou       = "Não consegui guardar a chave neste computador."
)

// Atrás de variáveis para os testes não dependerem da máquina de verdade.
var (
	detectarOrigem = origem.Detectar
	chaveValida    = guarda.Valida
	lerCodigo      = codigo.Ler
	conferirChave  = chave.Conferir
	gravarChave    = guarda.Gravar
	pastaCache     = func() string {
		home, _ := os.UserHomeDir()
		return cache.Pasta(runtime.GOOS, os.Getenv, home)
	}
)

// deveTrancar diz se a IA fica trancada: só a cópia instalada sem chave
// válida. O pen drive e a dúvida sobem livres, para nunca trancar o teste,
// e nem chegam a ler a chave.
func deveTrancar(o origem.Origem, valida func() bool) bool {
	return o == origem.Instalada && !valida()
}

// itensBandejaTrancada é o menu do ícone com a IA trancada: não há o que
// pôr para dormir, só mostrar a janela ou fechar.
func itensBandejaTrancada() []item {
	return []item{{itemMostrar, true}, {itemFechar, true}}
}

// estaTrancada diz se a IA está esperando a chave.
func (a *App) estaTrancada() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.trancada
}

// fecharSai diz se fechar a janela encerra o app: com o erro na tela ou
// trancada, não há IA acordada para ficar no ícone.
func (a *App) fecharSai() bool {
	return a.estaTrancada() || fecharVaiDormir(a.s.Atual().Fase)
}

// ColarChave é o "Liberar" da tela trancada. Devolve "" se a chave vale para
// este computador (aí grava e acorda a IA; o próximo estado troca a tela) ou
// o texto do problema.
func (a *App) ColarChave(k string) string {
	norm, ok := chave.Normalizar(k)
	if !ok {
		return textoChaveIncompleta
	}
	cod, err := lerCodigo()
	if err != nil {
		return textoSemCodigo
	}
	if !conferirChave(norm, cod) {
		return textoChaveDeOutro
	}
	if err := gravarChave(pastaCache(), norm); err != nil {
		return textoNaoGravou
	}
	a.mu.Lock()
	era := a.trancada
	a.trancada = false
	a.mu.Unlock()
	if era {
		a.atualizarBandeja(a.s.Atual())
		a.acordar()
	}
	return ""
}
