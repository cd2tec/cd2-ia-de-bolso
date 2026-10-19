package webview2

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Na rede de alguns clientes a Roaming (o padrão do WebView2) não aceita
// escrita: a pasta escolhida tem de ser a primeira em que dá para gravar.
func TestEscolherPulaAQueNaoGrava(t *testing.T) {
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
	if got := Escolher([]string{"", filepath.Join(trancada, "webview2"), boa}); got != boa {
		t.Fatalf("pasta = %q, quero %q", got, boa)
	}
	if entradas, _ := os.ReadDir(boa); len(entradas) != 0 {
		t.Errorf("o teste de escrita deixou lixo: %v", entradas)
	}
}

// Sem nenhuma pasta boa, devolve vazio e o Wails fica com o padrão dele.
func TestEscolherSemNenhumaBoa(t *testing.T) {
	if got := Escolher([]string{"", ""}); got != "" {
		t.Fatalf("pasta = %q, quero vazio", got)
	}
}

// Cada app tem a sua pasta na Local do cd2-harness, nunca na Roaming.
func TestCandidatas(t *testing.T) {
	env := func(k string) string {
		return map[string]string{"LOCALAPPDATA": `C:\U\AppData\Local`, "TEMP": `C:\T`, "APPDATA": `C:\U\AppData\Roaming`}[k]
	}
	got := Candidatas(env, "instalador")
	want := []string{`C:\U\AppData\Local\cd2-harness\webview2\instalador`, `C:\T\cd2-harness\webview2\instalador`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidatas = %q, quero %q", got, want)
	}
}

// Variável de ambiente vazia não vira caminho relativo solto.
func TestCandidatasSemVariaveis(t *testing.T) {
	if got := Candidatas(func(string) string { return "" }, "lancador"); len(got) != 0 {
		t.Fatalf("candidatas = %q, quero nenhuma", got)
	}
}
