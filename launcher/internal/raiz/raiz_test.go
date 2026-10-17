package raiz

import "testing"

func TestRaizMac(t *testing.T) {
	r, err := DoExecutavel("/Volumes/CD2 1/IA de Bolso.app/Contents/MacOS/IA de Bolso", "darwin")
	if err != nil || r != "/Volumes/CD2 1" {
		t.Fatalf("raiz = %q, err = %v", r, err)
	}
}

func TestRaizMacForaDoApp(t *testing.T) {
	if _, err := DoExecutavel("/tmp/IA de Bolso", "darwin"); err == nil {
		t.Fatal("esperava erro fora de um .app")
	}
}

func TestRaizWindows(t *testing.T) {
	r, err := DoExecutavel(`E:\IA de Bolso.exe`, "windows")
	if err != nil || r != `E:\` {
		t.Fatalf("raiz = %q, err = %v", r, err)
	}
}

func TestRaizWindowsSubpasta(t *testing.T) {
	r, err := DoExecutavel(`E:\CD2\IA de Bolso.exe`, "windows")
	if err != nil || r != `E:\CD2` {
		t.Fatalf("raiz = %q, err = %v", r, err)
	}
}
