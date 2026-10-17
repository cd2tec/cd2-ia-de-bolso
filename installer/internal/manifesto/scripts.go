package manifesto

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
)

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

// Fixos são os valores que ficam escritos nos scripts de preparo do pen drive.
type Fixos struct {
	ModeloURL, ModeloNome, ModeloSHA256, LlamafileReserva string
	ToolboxVersao, ToolboxBase, CtlVersao, CtlBase        string
	Plataformas                                           []string
}

func achar(texto, quem string, re *regexp.Regexp) (string, error) {
	m := re.FindStringSubmatch(texto)
	if m == nil {
		return "", fmt.Errorf("%s: não achei %s", quem, re)
	}
	return m[1], nil
}

// DosScripts lê os valores fixos de prepare-unix.sh e prepare-windows.ps1 (o conteúdo
// dos arquivos, não o caminho) e exige que os dois concordem entre si.
func DosScripts(prepareUnix, prepareWindows string) (Fixos, error) {
	var f Fixos
	var erros []string
	sh := func(nome string) string {
		// NOME="${CD2_X:-valor}"  (o padrão dentro de ${...:-...})
		re := regexp.MustCompile(`(?m)^\s*(?:else\s+)?` + nome + `="\$\{[A-Z0-9_]+:-([^}]*)\}"`)
		v, err := achar(prepareUnix, "prepare-unix.sh", re)
		if err != nil {
			erros = append(erros, err.Error())
		}
		return v
	}
	shFixo := func(nome string) string {
		re := regexp.MustCompile(`(?m)^\s*` + nome + `="([^"]*)"`)
		v, err := achar(prepareUnix, "prepare-unix.sh", re)
		if err != nil {
			erros = append(erros, err.Error())
		}
		return v
	}
	ps := func(nome string) string {
		// $Nome = if (...) { ... } else { 'valor' }   (o último literal entre aspas simples)
		re := regexp.MustCompile(`(?m)^\$` + nome + `\s*=.*else\s*\{\s*'([^']*)'\s*\}\s*$`)
		v, err := achar(prepareWindows, "prepare-windows.ps1", re)
		if err != nil {
			erros = append(erros, err.Error())
		}
		return v
	}
	psFixo := func(nome string) string {
		re := regexp.MustCompile(`(?m)^\$` + nome + `\s*=\s*'([^']*)'`)
		v, err := achar(prepareWindows, "prepare-windows.ps1", re)
		if err != nil {
			erros = append(erros, err.Error())
		}
		return v
	}

	f.ModeloURL = sh("MODEL_URL")
	f.ModeloNome = sh("MODEL_FILE")
	f.ModeloSHA256 = sh("MODEL_SHA256")
	f.LlamafileReserva = shFixo("FALLBACK_BIN_URL")
	f.ToolboxVersao = sh("TOOLBOX_VERSION")
	f.CtlVersao = sh("CTL_VERSION")
	toolboxBase := shFixo("TOOLBOX_BASE")
	ctlBase := shFixo("CTL_BASE")
	plat := sh("TOOLBOX_PLATFORMS")
	f.Plataformas = strings.Fields(plat)
	f.ToolboxBase = strings.ReplaceAll(toolboxBase, "$TOOLBOX_VERSION", f.ToolboxVersao)
	f.CtlBase = strings.ReplaceAll(ctlBase, "$CTL_VERSION", f.CtlVersao)

	wModeloURL := ps("ModelUrl")
	wModeloNome := ps("ModelFile")
	wModeloSHA := ps("ModelSha")
	wReserva := psFixo("FallbackBin")
	wToolbox := ps("ToolboxVersion")
	wCtl := ps("CtlVersion")
	var wPlat []string
	rePlat := regexp.MustCompile(`(?m)^\$ToolboxPlatforms\s*=.*else\s*\{\s*@\(([^)]*)\)\s*\}`)
	if lista, err := achar(prepareWindows, "prepare-windows.ps1", rePlat); err != nil {
		erros = append(erros, err.Error())
	} else {
		for _, p := range strings.Split(lista, ",") {
			if p = strings.Trim(strings.TrimSpace(p), `'`); p != "" {
				wPlat = append(wPlat, p)
			}
		}
	}

	if len(erros) > 0 {
		return Fixos{}, fmt.Errorf("valores ausentes nos scripts: %s", strings.Join(erros, "; "))
	}

	iguais := func(nome string, a, b any) {
		if !reflect.DeepEqual(a, b) {
			erros = append(erros, fmt.Sprintf("%s diverge: unix=%v windows=%v", nome, a, b))
		}
	}
	iguais("modelo (url)", f.ModeloURL, wModeloURL)
	iguais("modelo (nome)", f.ModeloNome, wModeloNome)
	iguais("modelo (sha256)", f.ModeloSHA256, strings.ToLower(wModeloSHA))
	iguais("llamafile (reserva)", f.LlamafileReserva, wReserva)
	iguais("toolbox (versão)", f.ToolboxVersao, wToolbox)
	iguais("toolbox (plataformas)", f.Plataformas, wPlat)
	iguais("ctl (versão)", f.CtlVersao, wCtl)
	if len(erros) > 0 {
		return Fixos{}, fmt.Errorf("prepare-unix.sh e prepare-windows.ps1 divergem: %s", strings.Join(erros, "; "))
	}
	return f, nil
}
