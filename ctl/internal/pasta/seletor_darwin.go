package pasta

import (
	"os/exec"
	"strings"
)

// O AppleScript recebe a pasta inicial como argumento (nunca dentro do texto do script).
// "activate" traz o diálogo para a frente do navegador.
var scriptMac = []string{
	"on run argv",
	"activate",
	"set p to \"Escolha a pasta\"",
	"if (item 1 of argv) is \"\" then",
	"return POSIX path of (choose folder with prompt p)",
	"end if",
	"return POSIX path of (choose folder with prompt p default location (POSIX file (item 1 of argv)))",
	"end run",
}

func seletor(inicial string) (comando, error) {
	var args []string
	for _, l := range scriptMac {
		args = append(args, "-e", l)
	}
	args = append(args, inicial)
	// Cancelar no choose folder é o erro -128 ("User canceled").
	return comando{nome: "osascript", args: args, cancelou: func(_ int, stderr string) bool {
		return strings.Contains(stderr, "-128")
	}}, nil
}

func esconder(*exec.Cmd) {}
