package main

import (
	"os"
	"runtime"
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

// notifyIconData é o NOTIFYICONDATAW, com o mesmo layout que o fyne.io/systray usa.
type notifyIconData struct {
	Size                       uint32
	Wnd                        windows.Handle
	ID, Flags, CallbackMessage uint32
	Icon                       windows.Handle
	Tip                        [128]uint16
	State, StateMask           uint32
	Info                       [256]uint16
	Timeout, Version           uint32
	InfoTitle                  [64]uint16
	InfoFlags                  uint32
	GuidItem                   windows.GUID
	BalloonIcon                windows.Handle
}

var (
	shell32         = windows.NewLazySystemDLL("shell32.dll")
	user32          = windows.NewLazySystemDLL("user32.dll")
	pShellNotify    = shell32.NewProc("Shell_NotifyIconW")
	pFindWindowEx   = user32.NewProc("FindWindowExW")
	pGetWindowPidOf = user32.NewProc("GetWindowThreadProcessId")
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

// avisarPrimeiroFechar mostra a notificação do sistema pelo próprio ícone da
// bandeja (NIF_INFO no mesmo ícone, id 100, que é o que o systray usa). O
// fyne.io/systray não tem balão; isto evita uma dependência nova só para ele.
func avisarPrimeiroFechar() {
	const (
		nimModify = 0x1
		nifInfo   = 0x10
		niifInfo  = 0x1
		idIcone   = 100
	)
	h := janelaDoSystray()
	if h == 0 {
		return
	}
	nid := notifyIconData{Wnd: h, ID: idIcone, Flags: nifInfo, InfoFlags: niifInfo}
	nid.Size = uint32(unsafe.Sizeof(nid))
	info, _ := windows.UTF16FromString(textoAvisoBandeja)
	titulo, _ := windows.UTF16FromString(nomeBandeja)
	copy(nid.Info[:len(nid.Info)-1], info)
	copy(nid.InfoTitle[:len(nid.InfoTitle)-1], titulo)
	pShellNotify.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}
