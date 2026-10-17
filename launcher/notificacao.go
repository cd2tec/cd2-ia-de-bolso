package main

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
