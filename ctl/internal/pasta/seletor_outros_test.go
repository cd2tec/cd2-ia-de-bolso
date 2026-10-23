//go:build !darwin && !windows

package pasta

import (
	"errors"
	"os/exec"
	"testing"
)

func comProgramas(t *testing.T, tem ...string) {
	t.Helper()
	antes := lookPath
	t.Cleanup(func() { lookPath = antes })
	lookPath = func(n string) (string, error) {
		for _, x := range tem {
			if x == n {
				return "/usr/bin/" + n, nil
			}
		}
		return "", exec.ErrNotFound
	}
}

func TestSeletorLinux(t *testing.T) {
	comProgramas(t, "zenity", "kdialog")
	c, err := seletor("/home/a")
	if err != nil || c.nome != "/usr/bin/zenity" || c.args[len(c.args)-1] != "--filename=/home/a/" || !c.cancelou(1, "") {
		t.Fatalf("zenity: %+v %v", c, err)
	}
	comProgramas(t, "kdialog")
	c, err = seletor("/home/a")
	if err != nil || c.nome != "/usr/bin/kdialog" || c.args[len(c.args)-1] != "/home/a" {
		t.Fatalf("kdialog: %+v %v", c, err)
	}
	comProgramas(t)
	if _, err = seletor("/home/a"); !errors.Is(err, ErrSemSeletor) {
		t.Fatalf("sem seletor: %v", err)
	}
}
