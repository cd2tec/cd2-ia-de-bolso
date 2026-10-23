//go:build !darwin && !windows

package pasta

import "os/exec"

// Linux e afins: zenity (GNOME e a maioria) e, sem ele, kdialog (KDE). Os dois saem com 1 no Cancelar.
func seletor(inicial string) (comando, error) {
	if z, err := lookPath("zenity"); err == nil {
		args := []string{"--file-selection", "--directory", "--title=Escolha a pasta"}
		if inicial != "" {
			args = append(args, "--filename="+inicial+"/")
		}
		return comando{nome: z, args: args, cancelou: porCodigo(1)}, nil
	}
	if k, err := lookPath("kdialog"); err == nil {
		args := []string{"--title", "Escolha a pasta", "--getexistingdirectory"}
		if inicial != "" {
			args = append(args, inicial)
		}
		return comando{nome: k, args: args, cancelou: porCodigo(1)}, nil
	}
	return comando{}, ErrSemSeletor
}

func esconder(*exec.Cmd) {}
