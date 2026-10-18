package manifesto

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

const (
	caminhoUnix    = "../../../core/prepare-unix.sh"
	caminhoWindows = "../../../core/prepare-windows.ps1"
)

func lerArquivo(t *testing.T, caminho string) string {
	t.Helper()
	b, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDosScriptsLeOsValoresAtuais(t *testing.T) {
	f, err := DosScripts(lerArquivo(t, caminhoUnix), lerArquivo(t, caminhoWindows))
	if err != nil {
		t.Fatal(err)
	}
	if f.ModeloNome != "Qwen3-4B-Instruct-2507-Q4_K_M.gguf" {
		t.Errorf("ModeloNome = %q", f.ModeloNome)
	}
	if f.ModeloSHA256 != "3605803b982cb64aead44f6c1b2ae36e3acdb41d8e46c8a94c6533bc4c67e597" {
		t.Errorf("ModeloSHA256 = %q", f.ModeloSHA256)
	}
	if f.ModeloURL != "https://huggingface.co/unsloth/Qwen3-4B-Instruct-2507-GGUF/resolve/main/Qwen3-4B-Instruct-2507-Q4_K_M.gguf" {
		t.Errorf("ModeloURL = %q", f.ModeloURL)
	}
	if f.LlamafileURL != "https://github.com/mozilla-ai/llamafile/releases/download/0.10.6/llamafile-0.10.6" {
		t.Errorf("LlamafileURL = %q", f.LlamafileURL)
	}
	if f.LlamafileSHA256 != "d579f61dcd3a306f518e6d90e599d77793ed5f09543023d09c96ad35fcfa63f0" {
		t.Errorf("LlamafileSHA256 = %q", f.LlamafileSHA256)
	}
	if f.ToolboxVersao != "1.14.0" {
		t.Errorf("ToolboxVersao = %q", f.ToolboxVersao)
	}
	if f.ToolboxBase != "https://storage.googleapis.com/mcp-toolbox-for-databases/v1.14.0" {
		t.Errorf("ToolboxBase = %q", f.ToolboxBase)
	}
	if f.CtlVersao != "0.6.0" {
		t.Errorf("CtlVersao = %q", f.CtlVersao)
	}
	if f.CtlBase != "https://github.com/cd2tec/cd2-ia-de-bolso/releases/download/ctl-v0.6.0" {
		t.Errorf("CtlBase = %q", f.CtlBase)
	}
	quero := []string{"windows-amd64", "darwin-arm64", "darwin-amd64", "linux-amd64"}
	if !reflect.DeepEqual(f.Plataformas, quero) {
		t.Errorf("Plataformas = %v", f.Plataformas)
	}
}

func TestDosScriptsRecusaDivergencia(t *testing.T) {
	ps1 := strings.Replace(lerArquivo(t, caminhoWindows), "'1.14.0'", "'1.15.0'", 1)
	_, err := DosScripts(lerArquivo(t, caminhoUnix), ps1)
	if err == nil {
		t.Fatal("esperava erro de divergencia")
	}
	if !strings.Contains(err.Error(), "toolbox") {
		t.Fatalf("o erro deveria citar toolbox: %v", err)
	}
}

func TestDosScriptsRecusaValorAusente(t *testing.T) {
	sh := strings.Replace(lerArquivo(t, caminhoUnix), "CTL_VERSION=", "OUTRA_COISA=", 1)
	if _, err := DosScripts(sh, lerArquivo(t, caminhoWindows)); err == nil {
		t.Fatal("esperava erro com valor ausente")
	}
}

func TestDosScriptsRecusaMotorDivergente(t *testing.T) {
	ps1 := strings.Replace(lerArquivo(t, caminhoWindows), "d579f61dcd3a", "0000000000aa", 1)
	_, err := DosScripts(lerArquivo(t, caminhoUnix), ps1)
	if err == nil || !strings.Contains(err.Error(), "llamafile") {
		t.Fatalf("esperava erro citando llamafile: %v", err)
	}
}
