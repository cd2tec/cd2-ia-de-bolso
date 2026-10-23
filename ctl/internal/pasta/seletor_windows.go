package pasta

import (
	"os/exec"
	"syscall"
)

// A pasta inicial vai pela variável CD2_PASTA_INICIAL (nunca dentro do texto do script). O
// formulário dono com TopMost põe o diálogo na frente do navegador. Cancelar sai com 3.
const scriptWindows = `Add-Type -AssemblyName System.Windows.Forms
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$d = New-Object System.Windows.Forms.FolderBrowserDialog
$d.Description = 'Escolha a pasta'
$d.ShowNewFolderButton = $false
if ($env:CD2_PASTA_INICIAL) { $d.SelectedPath = $env:CD2_PASTA_INICIAL }
$o = New-Object System.Windows.Forms.Form
$o.TopMost = $true
if ($d.ShowDialog($o) -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::Out.Write($d.SelectedPath) } else { exit 3 }`

func seletor(inicial string) (comando, error) {
	return comando{
		nome:     "powershell.exe",
		args:     []string{"-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-Command", scriptWindows},
		env:      []string{"CD2_PASTA_INICIAL=" + inicial},
		cancelou: porCodigo(3),
	}, nil
}

// esconder: sem janela de console do PowerShell piscando atrás do diálogo.
func esconder(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
