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

func atalhoMenu() string {
	return ambiente("APPDATA") + `\Microsoft\Windows\Start Menu\Programs\` + nomeAtalho
}

// powershell monta o Comando que roda o script. Os caminhos vão em env e o
// script os lê por $env:NOME, sem nenhum caminho dentro do texto.
func powershell(script string, env ...string) Comando {
	return Comando{
		Exe:  "powershell.exe",
		Args: []string{"-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script},
		Env:  env,
	}
}

// A área de trabalho vem do GetFolderPath, que acerta quando o OneDrive a
// mudou de lugar.
const linhaMesa = "$mesa = Join-Path ([Environment]::GetFolderPath('Desktop')) '" + nomeAtalho + "'; "

// comandoAtalhos monta o PowerShell que cria os atalhos com o WScript.Shell.
func comandoAtalhos(lancador, menu string, areaDeTrabalho bool) Comando {
	var b strings.Builder
	b.WriteString("$ErrorActionPreference = 'Stop'; ")
	b.WriteString("$w = New-Object -ComObject WScript.Shell; ")
	b.WriteString(linhaMesa)
	alvos := []string{"$env:CD2_MENU"}
	if areaDeTrabalho {
		alvos = append(alvos, "$mesa")
	}
	for _, alvo := range alvos {
		b.WriteString("$a = $w.CreateShortcut(" + alvo + "); ")
		b.WriteString("$a.TargetPath = $env:CD2_LANCADOR; ")
		b.WriteString("$a.WorkingDirectory = $env:CD2_PASTA; ")
		b.WriteString("$a.IconLocation = $env:CD2_LANCADOR + ',0'; ")
		b.WriteString("$a.Save(); ")
	}
	if !areaDeTrabalho {
		b.WriteString("Remove-Item -LiteralPath $mesa -Force -ErrorAction SilentlyContinue; ")
	}
	return powershell(b.String(),
		"CD2_MENU="+menu, "CD2_LANCADOR="+lancador, "CD2_PASTA="+pai(lancador, `\`))
}

// comandoRemoverAtalhos monta o PowerShell que tira os dois atalhos.
func comandoRemoverAtalhos(menu string) Comando {
	return powershell(linhaMesa+
		"Remove-Item -LiteralPath $env:CD2_MENU -Force -ErrorAction SilentlyContinue; "+
		"Remove-Item -LiteralPath $mesa -Force -ErrorAction SilentlyContinue",
		"CD2_MENU="+menu)
}

func (windows) Atalhos(lancador string, areaDeTrabalho bool) error {
	if err := rodar(comandoAtalhos(lancador, atalhoMenu(), areaDeTrabalho)); err != nil {
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
	if err := rodar(comandoRemoverAtalhos(atalhoMenu())); err != nil {
		return fmt.Errorf("não consegui tirar os atalhos: %w", err)
	}
	return nil
}

// destinoWindowsValido aceita só um caminho absoluto (C:\... ou \\...) que
// termina na pasta da instalação.
func destinoWindowsValido(d string) bool {
	abs := strings.HasPrefix(d, `\\`) ||
		(len(d) >= 3 && d[1] == ':' && d[2] == '\\' && (d[0]|0x20) >= 'a' && (d[0]|0x20) <= 'z')
	return abs && base(d, `\`) == pastaWindows
}

// PlanoRemocao espera até 30 s o lançador sair e apaga a pasta, tudo em
// PowerShell (o timeout do cmd falha sem console). Sai da pasta antes,
// porque não dá para apagar a pasta em que se está.
func (windows) PlanoRemocao(destino string, pid int) Comando {
	if !destinoWindowsValido(destino) || pid <= 0 {
		return Comando{}
	}
	return powershell(fmt.Sprintf("Wait-Process -Id %d -Timeout 30 -ErrorAction SilentlyContinue; "+
		"Set-Location -LiteralPath $env:TEMP; "+
		"Remove-Item -LiteralPath $env:CD2_DESTINO -Recurse -Force -ErrorAction SilentlyContinue", pid),
		"CD2_DESTINO="+destino)
}
