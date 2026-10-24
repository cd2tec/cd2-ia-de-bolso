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

// O texto do balão cabe no campo com o NUL do fim, sem partir um emoji (par
// substituto do UTF-16) ao meio.
func TestUTF16Cabendo(t *testing.T) {
	casos := []struct {
		texto string
		n     int
		quer  []uint16
	}{
		{"oi", 8, []uint16{'o', 'i', 0}},
		{"abcd", 3, []uint16{'a', 'b', 0}},
		{"ação", 5, []uint16{'a', 0xe7, 0xe3, 'o', 0}},
		{"a😀", 3, []uint16{'a', 0}},
		{"a😀", 4, []uint16{'a', 0xd83d, 0xde00, 0}},
		{"", 4, []uint16{0}},
	}
	for _, c := range casos {
		got := utf16Cabendo(c.texto, c.n)
		if len(got) != len(c.quer) {
			t.Errorf("utf16Cabendo(%q, %d) = %x, quer %x", c.texto, c.n, got, c.quer)
			continue
		}
		for i := range got {
			if got[i] != c.quer[i] {
				t.Errorf("utf16Cabendo(%q, %d) = %x, quer %x", c.texto, c.n, got, c.quer)
				break
			}
		}
	}
}

// O clique no balão chega como NIN_BALLOONUSERCLICK no lParam da mensagem de
// callback do systray (WM_USER+1).
func TestCliqueNoBalao(t *testing.T) {
	casos := []struct {
		msg    uint32
		lParam uintptr
		quer   bool
	}{
		{0x401, 0x405, true},
		{0x401, 0x202, false}, // WM_LBUTTONUP: o clique no ícone
		{0x401, 0x404, false}, // NIN_BALLOONTIMEOUT
		{0x111, 0x405, false}, // WM_COMMAND
	}
	for _, c := range casos {
		if got := cliqueNoBalao(c.msg, c.lParam); got != c.quer {
			t.Errorf("cliqueNoBalao(%#x, %#x) = %v, quer %v", c.msg, c.lParam, got, c.quer)
		}
	}
}
