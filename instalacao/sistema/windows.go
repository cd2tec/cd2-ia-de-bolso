package sistema

import (
	"fmt"
	"strings"
)

// Este arquivo não tem build tag de propósito: só monta textos e valores, e
// os testes conferem tudo em qualquer máquina. O acesso de verdade ao
// registro fica em registro_windows.go.

const (
	nomeAtalho   = "CD2 IA de Bolso.lnk"
	chaveRun     = `Software\Microsoft\Windows\CurrentVersion\Run`
	nomeRun      = "CD2 IA de Bolso"
	chaveDesinst = `Software\Microsoft\Windows\CurrentVersion\Uninstall\CD2IADeBolso`
)

// regUsuario grava e apaga valores em HKEY_CURRENT_USER. As chaves são
// relativas a HKCU.
type regUsuario interface {
	GravarTexto(chave, nome, valor string) error
	GravarNumero(chave, nome string, valor uint32) error
	Existe(chave, nome string) bool
	ApagarValor(chave, nome string) error // não reclama se não existe
	ApagarChave(chave string) error       // não reclama se não existe
}

// registro fica em variável para os testes trocarem.
var registro regUsuario = registroDoSistema{}

type windows struct{}

// ps escreve s como texto literal do PowerShell, entre aspas simples.
func ps(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func atalhoMenu() string {
	return ambiente("APPDATA") + `\Microsoft\Windows\Start Menu\Programs\` + nomeAtalho
}

// scriptAtalhos monta o PowerShell que cria os atalhos com o WScript.Shell.
// A área de trabalho vem do GetFolderPath, que acerta quando o OneDrive a
// mudou de lugar.
func scriptAtalhos(lancador, menu string, areaDeTrabalho bool) string {
	pasta := pai(lancador, `\`)
	var b strings.Builder
	b.WriteString("$ErrorActionPreference = 'Stop'; ")
	b.WriteString("$w = New-Object -ComObject WScript.Shell; ")
	b.WriteString("$mesa = Join-Path ([Environment]::GetFolderPath('Desktop')) " + ps(nomeAtalho) + "; ")
	alvos := []string{ps(menu)}
	if areaDeTrabalho {
		alvos = append(alvos, "$mesa")
	}
	for _, alvo := range alvos {
		b.WriteString("$a = $w.CreateShortcut(" + alvo + "); ")
		b.WriteString("$a.TargetPath = " + ps(lancador) + "; ")
		b.WriteString("$a.WorkingDirectory = " + ps(pasta) + "; ")
		b.WriteString("$a.IconLocation = " + ps(lancador+",0") + "; ")
		b.WriteString("$a.Save(); ")
	}
	if !areaDeTrabalho {
		b.WriteString("Remove-Item -LiteralPath $mesa -Force -ErrorAction SilentlyContinue; ")
	}
	return b.String()
}

// scriptRemoverAtalhos monta o PowerShell que tira os dois atalhos.
func scriptRemoverAtalhos(menu string) string {
	return "$mesa = Join-Path ([Environment]::GetFolderPath('Desktop')) " + ps(nomeAtalho) + "; " +
		"Remove-Item -LiteralPath " + ps(menu) + " -Force -ErrorAction SilentlyContinue; " +
		"Remove-Item -LiteralPath $mesa -Force -ErrorAction SilentlyContinue"
}

func powershell(script string) error {
	return rodar("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
}

func (windows) Atalhos(lancador string, areaDeTrabalho bool) error {
	if err := powershell(scriptAtalhos(lancador, atalhoMenu(), areaDeTrabalho)); err != nil {
		return fmt.Errorf("não consegui criar os atalhos: %w", err)
	}
	return nil
}

func (windows) LigarInicio(lancador string, ligar bool) error {
	var err error
	if ligar {
		err = registro.GravarTexto(chaveRun, nomeRun, `"`+lancador+`" --inicio`)
	} else {
		err = registro.ApagarValor(chaveRun, nomeRun)
	}
	if err != nil {
		return fmt.Errorf("não consegui mudar o início com o computador: %w", err)
	}
	return nil
}

func (windows) InicioLigado() bool {
	return registro.Existe(chaveRun, nomeRun)
}

func (windows) Registrar(lancador, versao string) error {
	textos := []struct{ nome, valor string }{
		{"DisplayName", "CD2 IA de Bolso"},
		{"DisplayVersion", versao},
		{"Publisher", "CD2 Tech"},
		{"UninstallString", `"` + lancador + `" --desinstalar`},
		{"DisplayIcon", `"` + lancador + `"`},
		{"InstallLocation", pai(lancador, `\`)},
	}
	for _, v := range textos {
		if err := registro.GravarTexto(chaveDesinst, v.nome, v.valor); err != nil {
			return fmt.Errorf("não consegui registrar a instalação: %w", err)
		}
	}
	if err := registro.GravarNumero(chaveDesinst, "NoModify", 1); err != nil {
		return fmt.Errorf("não consegui registrar a instalação: %w", err)
	}
	return nil
}

func (windows) Remover() error {
	if err := registro.ApagarChave(chaveDesinst); err != nil {
		return fmt.Errorf("não consegui tirar o registro da instalação: %w", err)
	}
	if err := powershell(scriptRemoverAtalhos(atalhoMenu())); err != nil {
		return fmt.Errorf("não consegui tirar os atalhos: %w", err)
	}
	return nil
}

// PlanoRemocao espera até 30 s o lançador sair (tasklist em laço) e apaga a
// pasta. Sai da pasta antes, porque o cmd não apaga a pasta em que está.
func (windows) PlanoRemocao(destino string, pid int) Comando {
	p := fmt.Sprint(pid)
	linha := `cd /d "%TEMP%" & (for /l %i in (1,1,30) do @(tasklist /FI "PID eq ` + p + `" /NH | find " ` + p +
		` " >nul && timeout /t 1 /nobreak >nul)) & rmdir /s /q "` + destino + `"`
	return Comando{Exe: "cmd.exe", Args: []string{"/c", linha}}
}
