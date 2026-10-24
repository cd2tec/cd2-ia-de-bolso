package main

import "unicode/utf16"

// notifyIconData é o NOTIFYICONDATAW do Windows (shellapi.h). Fica sem build
// tag, com tipos neutros, para o teste de layout rodar em qualquer sistema de
// 64 bits; quem usa é o bandeja_windows.go.
type notifyIconData struct {
	Size                       uint32
	Wnd                        uintptr // HWND
	ID, Flags, CallbackMessage uint32
	Icon                       uintptr // HICON
	Tip                        [128]uint16
	State, StateMask           uint32
	Info                       [256]uint16
	TimeoutOuVersao            uint32 // union uTimeout/uVersion: um DWORD só
	InfoTitle                  [64]uint16
	InfoFlags                  uint32
	GuidItem                   guid
	BalloonIcon                uintptr // HICON
}

// guid tem o layout do GUID do Windows (alinhamento de 4).
type guid struct {
	Data1        uint32
	Data2, Data3 uint16
	Data4        [8]byte
}

// Mensagem de callback do ícone do fyne.io/systray (WM_USER+1, conferido na
// v1.12.2) e o aviso de clique no balão. O systray não pede a versão 4 do
// ícone, então o lParam é o evento puro.
const (
	wmSystray           = 0x0400 + 1
	ninBalloonUserClick = 0x405
)

// cliqueNoBalao diz se a mensagem é o clique num balão do ícone.
func cliqueNoBalao(msg uint32, lParam uintptr) bool {
	return msg == wmSystray && lParam == ninBalloonUserClick
}

// utf16Cabendo devolve o texto em UTF-16 com o NUL do fim, em no máximo n
// unidades (o tamanho do campo), sem partir um par substituto ao meio.
func utf16Cabendo(s string, n int) []uint16 {
	u := utf16.Encode([]rune(s))
	if len(u) > n-1 {
		u = u[:n-1]
		if len(u) > 0 && utf16.IsSurrogate(rune(u[len(u)-1])) && u[len(u)-1] < 0xdc00 {
			u = u[:len(u)-1]
		}
	}
	return append(u, 0)
}
