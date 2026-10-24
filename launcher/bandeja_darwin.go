package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework UserNotifications
#include <stdlib.h>
void agendarNaThreadPrincipal(void);
void trazerParaFrente(void);
void notificar(const char *titulo, const char *texto, const char *consulta);
*/
import "C"

import (
	"unsafe"

	"fyne.io/systray"
)

// No Mac o X da janela só esconde (HideWindowOnClose); o OnBeforeClose é o
// app saindo de verdade (Cmd+Q, logout).
const escondePeloAntesDeFechar = false

var (
	appBandeja     *App
	comecarBandeja func()
)

// iniciarBandeja põe o ícone na barra de menus. O NSApp é do Wails: o
// RunWithExternalLoop só registra, e o start (que cria o NSStatusItem)
// precisa rodar na thread principal, então vai por dispatch_async. O end
// nunca é chamado: ele faz [NSApp stop:] e espera a thread principal, e o
// Quit do Wails já encerra limpo sem ele.
func iniciarBandeja(a *App) {
	appBandeja = a
	start, _ := systray.RunWithExternalLoop(a.montarMenu, nil)
	comecarBandeja = start
	C.agendarNaThreadPrincipal()
}

//export bandejaNaThreadPrincipal
func bandejaNaThreadPrincipal() { comecarBandeja() }

// appReaberto é o dois cliques no .app que já está aberto: o LaunchServices
// não abre outra cópia, só avisa a que está rodando.
//
//export appReaberto
func appReaberto() {
	if appBandeja != nil {
		go appBandeja.chamar() // fora da thread principal: chamar fala com o Wails
	}
}

// appEscondido é o NSApplicationDidHideNotification: com HideWindowOnClose,
// o X da janela esconde o app inteiro, e só por aqui dá para saber.
//
//export appEscondido
func appEscondido() {
	if appBandeja != nil {
		go appBandeja.janelaFechada() // fora da thread principal: pode falar com o Wails
	}
}

// No Mac o clique no ícone abre o menu, que é o padrão do systray.
func ligarCliqueSimples(*App) {}

// A barra de menus do Mac aceita PNG.
func formatoIcone(png []byte) []byte { return png }

// Com o app sem ícone no Dock, a janela não vem para a frente sozinha.
func trazerParaFrente() { C.trazerParaFrente() }

func avisarPrimeiroFechar() bool { return false }

func fecharBandeja() {}

// No Mac a marca é o número ao lado do ícone (SetTitle), não um desenho.
const marcaNoIcone = false

// notificar mostra o aviso de um agendamento na Central de Notificações. O
// clique volta por notificacaoClicada.
func notificar(titulo, texto, consulta string) {
	ct, cx, cc := C.CString(titulo), C.CString(texto), C.CString(consulta)
	defer C.free(unsafe.Pointer(ct))
	defer C.free(unsafe.Pointer(cx))
	defer C.free(unsafe.Pointer(cc))
	C.notificar(ct, cx, cc)
}

// notificacaoClicada é o clique numa notificação: abre o histórico do
// agendamento.
//
//export notificacaoClicada
func notificacaoClicada(consulta *C.char) {
	c := C.GoString(consulta)
	if appBandeja != nil && c != "" {
		go appBandeja.abrirAgenda(c) // fora da thread principal: pode falar com o Wails
	}
}
