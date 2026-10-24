package main

import (
	"os"
	"runtime"
	"sync"
	"unsafe"

	"fyne.io/systray"
	"golang.org/x/sys/windows"

	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/icone"
)

// No Windows o X da janela passa pelo OnBeforeClose, que esconde a janela e
// avisa na primeira vez. O HideWindowOnClose do Wails esconderia sem avisar
// ninguém, e aí não daria para mandar a notificação.
const escondePeloAntesDeFechar = true

// Texto exato da notificação da primeira vez que a janelinha fecha.
const textoAvisoBandeja = "Continuo acordada aqui perto do relógio. Me chama quando quiser."

// iniciarBandeja põe o ícone perto do relógio. A bandeja tem o próprio laço
// de mensagens numa thread travada, sem disputar com o Wails. O onExit roda
// no logout (WM_ENDSESSION da janela do systray) e no systray.Quit: nos dois
// casos a IA dorme antes de o processo acabar.
func iniciarBandeja(a *App) {
	appBandeja = a
	go func() {
		runtime.LockOSThread()
		systray.Run(a.montarMenu, a.s.Dormir)
	}()
}

// No Windows o clique simples traz a janela; o botão direito abre o menu
// (sem função no SetOnSecondaryTapped, o systray mostra o menu). O chamar vai
// numa goroutine para não segurar o laço de mensagens da bandeja.
func ligarCliqueSimples(a *App) { systray.SetOnTapped(func() { go a.chamar() }) }

// A bandeja do Windows quer ICO.
func formatoIcone(png []byte) []byte { return icone.ICO(png) }

// O WindowShow do Wails já traz a janela para a frente.
func trazerParaFrente() {}

// O systray some no Quit; sem isso o ícone fica "fantasma" até passar o mouse.
func fecharBandeja() { systray.Quit() }

// A bandeja do Windows não mostra texto ao lado do ícone: a marca é o
// círculo vermelho no desenho.
const marcaNoIcone = true

// notificar mostra o aviso de um agendamento num balão do ícone. O clique no
// balão abre o histórico do agendamento.
func notificar(titulo, texto, consulta string) { mostrarBalao(titulo, texto, consulta) }

var (
	shell32         = windows.NewLazySystemDLL("shell32.dll")
	user32          = windows.NewLazySystemDLL("user32.dll")
	pShellNotify    = shell32.NewProc("Shell_NotifyIconW")
	pFindWindowEx   = user32.NewProc("FindWindowExW")
	pGetWindowPidOf = user32.NewProc("GetWindowThreadProcessId")
	pSetWindowLong  = user32.NewProc("SetWindowLongPtrW")
	pCallWindowProc = user32.NewProc("CallWindowProcW")
)

var (
	appBandeja *App

	// muBalao guarda a consulta do último balão e a WndProc original do
	// systray, que a subclasse chama para tudo que não é clique no balão.
	muBalao       sync.Mutex
	consultaBalao string
	procOriginal  uintptr
	procNova      uintptr
)

// janelaDoSystray acha a janela escondida que o fyne.io/systray cria (classe
// "SystrayClass") e que é dona do ícone, entre as deste processo.
func janelaDoSystray() windows.Handle {
	classe, _ := windows.UTF16PtrFromString("SystrayClass")
	var h uintptr
	for {
		h, _, _ = pFindWindowEx.Call(0, h, uintptr(unsafe.Pointer(classe)), 0)
		if h == 0 {
			return 0
		}
		var pid uint32
		pGetWindowPidOf.Call(h, uintptr(unsafe.Pointer(&pid)))
		if int(pid) == os.Getpid() {
			return windows.Handle(h)
		}
	}
}

// avisarPrimeiroFechar avisa que a IA continua na bandeja. Devolve se o
// aviso saiu (sem a janela do systray, fica para a próxima).
func avisarPrimeiroFechar() bool { return mostrarBalao(nomeBandeja, textoAvisoBandeja, "") }

// mostrarBalao mostra a notificação do sistema pelo próprio ícone da bandeja
// (NIF_INFO no mesmo ícone, id 100, que é o que o systray usa). O
// fyne.io/systray não tem balão; isto evita uma dependência nova só para ele.
// O consulta é o agendamento que o clique abre (vazio: o clique não faz nada).
func mostrarBalao(titulo, texto, consulta string) bool {
	const (
		nimModify = 0x1
		nifInfo   = 0x10
		niifInfo  = 0x1
		idIcone   = 100
	)
	h := janelaDoSystray()
	if h == 0 {
		return false
	}
	subclassificar(h)
	muBalao.Lock()
	consultaBalao = consulta
	muBalao.Unlock()
	nid := notifyIconData{Wnd: uintptr(h), ID: idIcone, Flags: nifInfo, InfoFlags: niifInfo}
	nid.Size = uint32(unsafe.Sizeof(nid))
	copy(nid.Info[:], utf16Cabendo(texto, len(nid.Info)))
	copy(nid.InfoTitle[:], utf16Cabendo(titulo, len(nid.InfoTitle)))
	ok, _, _ := pShellNotify.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
	return ok != 0
}

// subclassificar troca, uma vez só, a WndProc da janela do systray pela
// procBandeja, para saber do clique no balão. A mensagem chega na thread da
// bandeja; o resto segue para a WndProc do systray.
func subclassificar(h windows.Handle) {
	const gwlpWndProc = ^uintptr(3) // GWLP_WNDPROC (-4)
	muBalao.Lock()
	defer muBalao.Unlock()
	if procOriginal != 0 {
		return
	}
	if procNova == 0 {
		procNova = windows.NewCallback(procBandeja) // há um limite de callbacks: cria uma só
	}
	antiga, _, _ := pSetWindowLong.Call(uintptr(h), gwlpWndProc, procNova)
	procOriginal = antiga // 0 se falhou: tenta de novo no próximo balão
}

// procBandeja é a subclasse da janela do systray.
func procBandeja(h windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	muBalao.Lock()
	consulta, original := consultaBalao, procOriginal
	muBalao.Unlock()
	if cliqueNoBalao(msg, lParam) {
		if consulta != "" && appBandeja != nil {
			go appBandeja.abrirAgenda(consulta) // fora do laço de mensagens da bandeja
		}
		return 0
	}
	r, _, _ := pCallWindowProc.Call(original, uintptr(h), uintptr(msg), wParam, lParam)
	return r
}
