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
		if !strings.Contains(v, "INICIAR-CD2-Windows.bat") || !strings.Contains(v, "raiz do pen drive") {
			t.Errorf("%s não fala do INICIAR-CD2-Windows.bat na raiz do pen drive: %q", nome, v)
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
