package main

import (
	"testing"
	"unsafe"
)

// Offsets do NOTIFYICONDATAW em 64 bits, pela documentação do Windows
// (shellapi.h): uTimeout e uVersion são uma union de um DWORD só.
func TestNotifyIconDataLayout(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("layout conferido só em 64 bits")
	}
	var n notifyIconData
	casos := []struct {
		nome      string
		got, quer uintptr
	}{
		{"cbSize", unsafe.Sizeof(n), 976},
		{"hWnd", unsafe.Offsetof(n.Wnd), 8},
		{"uID", unsafe.Offsetof(n.ID), 16},
		{"uFlags", unsafe.Offsetof(n.Flags), 20},
		{"szTip", unsafe.Offsetof(n.Tip), 40},
		{"szInfo", unsafe.Offsetof(n.Info), 304},
		{"uTimeout/uVersion", unsafe.Offsetof(n.TimeoutOuVersao), 816},
		{"szInfoTitle", unsafe.Offsetof(n.InfoTitle), 820},
		{"dwInfoFlags", unsafe.Offsetof(n.InfoFlags), 948},
		{"guidItem", unsafe.Offsetof(n.GuidItem), 952},
		{"hBalloonIcon", unsafe.Offsetof(n.BalloonIcon), 968},
	}
	for _, c := range casos {
		if c.got != c.quer {
			t.Errorf("%s = %d, quer %d", c.nome, c.got, c.quer)
		}
	}
}
