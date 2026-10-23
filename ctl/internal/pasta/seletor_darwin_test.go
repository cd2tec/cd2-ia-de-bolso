package pasta

import (
	"os/exec"
	"strings"
	"testing"
)

func TestSeletorMac(t *testing.T) {
	c, err := seletor("/Users/a/Loja")
	if err != nil || c.nome != "osascript" || c.args[len(c.args)-1] != "/Users/a/Loja" {
		t.Fatalf("%+v %v", c, err)
	}
	if !c.cancelou(1, "execution error: User canceled. (-128)") || c.cancelou(1, "execution error: outra coisa (-1700)") {
		t.Fatal("cancelar do osascript")
	}
	// O AppleScript precisa compilar (sem abrir diálogo nenhum).
	src := strings.Join(scriptMac, "\n")
	out := t.TempDir() + "/s.scpt"
	cmd := exec.Command("osacompile", "-o", out)
	cmd.Stdin = strings.NewReader(src)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("osacompile: %v %s", err, b)
	}
}
