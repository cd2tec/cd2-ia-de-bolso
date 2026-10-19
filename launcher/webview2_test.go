package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// Toda mensagem do WebView2 sai em português: nenhum campo vazio (o Wails não
// completa com o padrão) e nenhum igual ao inglês do Wails.
func TestMensagensWebView2EmPortugues(t *testing.T) {
	m, padrao := reflect.ValueOf(*mensagensWebView2()), reflect.ValueOf(*windows.DefaultMessages())
	for i := 0; i < m.NumField(); i++ {
		nome, v := m.Type().Field(i).Name, m.Field(i).String()
		if strings.TrimSpace(v) == "" {
			t.Errorf("%s vazio", nome)
		}
		if v == padrao.Field(i).String() {
			t.Errorf("%s ficou em inglês: %q", nome, v)
		}
	}
}

// Sem WebView2 a janelinha não abre: a mensagem aponta o .bat da raiz.
func TestMensagensWebView2ApontamOBat(t *testing.T) {
	m := mensagensWebView2()
	for nome, v := range map[string]string{
		"InstallationRequired": m.InstallationRequired, "UpdateRequired": m.UpdateRequired,
		"Webview2NotInstalled": m.Webview2NotInstalled, "FailedToInstall": m.FailedToInstall,
		"ContactAdmin": m.ContactAdmin, "InvalidFixedWebview2": m.InvalidFixedWebview2,
		"WebView2ProcessCrash": m.WebView2ProcessCrash,
	} {
		if !strings.Contains(v, "INICIAR-CD2-Windows.bat") || !strings.Contains(v, "pasta core do pen drive") {
			t.Errorf("%s não fala do INICIAR-CD2-Windows.bat na pasta core do pen drive: %q", nome, v)
		}
	}
}

func TestRegistrarFalha(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	if err := registrarFalha(dir, "0.14.0", errors.New("primeira")); err != nil {
		t.Fatal(err)
	}
	if err := registrarFalha(dir, "0.14.0", errors.New("segunda")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "lancador.log"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "lançador 0.14.0: primeira") || !strings.Contains(s, "lançador 0.14.0: segunda") {
		t.Errorf("log sem as duas falhas: %q", s)
	}
}

// Na rede de alguns clientes a Roaming (o padrão do WebView2) não aceita
// escrita: a pasta escolhida tem de ser a primeira em que dá para gravar.
func TestPastaDadosWebView2PulaAQueNaoGrava(t *testing.T) {
	base := t.TempDir()
	trancada := filepath.Join(base, "trancada")
	if err := os.Mkdir(trancada, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(trancada, 0o700) })
	if f, err := os.Create(filepath.Join(trancada, "x")); err == nil {
		f.Close()
		t.Skip("rodando como root: a pasta trancada aceita escrita")
	}
	boa := filepath.Join(base, "boa", "webview2")
	if got := pastaDadosWebView2([]string{"", filepath.Join(trancada, "webview2"), boa}); got != boa {
		t.Fatalf("pasta = %q, quero %q", got, boa)
	}
	if entradas, _ := os.ReadDir(boa); len(entradas) != 0 {
		t.Errorf("o teste de escrita deixou lixo: %v", entradas)
	}
}

// Sem nenhuma pasta boa, devolve vazio e o Wails fica com o padrão dele.
func TestPastaDadosWebView2SemNenhumaBoa(t *testing.T) {
	if got := pastaDadosWebView2([]string{"", ""}); got != "" {
		t.Fatalf("pasta = %q, quero vazio", got)
	}
}

// No Windows, a primeira candidata é a pasta Local do cd2-harness, nunca a Roaming.
func TestCandidatasWebView2(t *testing.T) {
	env := func(k string) string {
		return map[string]string{"LOCALAPPDATA": `C:\U\AppData\Local`, "TEMP": `C:\T`, "APPDATA": `C:\U\AppData\Roaming`}[k]
	}
	got := candidatasWebView2(env, "")
	want := []string{`C:\U\AppData\Local\cd2-harness\webview2`, `C:\T\cd2-harness\webview2`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidatas = %q, quero %q", got, want)
	}
}

// Variável de ambiente vazia não vira caminho relativo solto.
func TestCandidatasWebView2SemVariaveis(t *testing.T) {
	if got := candidatasWebView2(func(string) string { return "" }, ""); len(got) != 0 {
		t.Fatalf("candidatas = %q, quero nenhuma", got)
	}
}
