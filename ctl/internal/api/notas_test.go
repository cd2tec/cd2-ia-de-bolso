package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/control"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/notas"
)

// backendNotas devolve o Status que o teste quiser.
type backendNotas struct {
	fakeBackend
	st control.Status
}

func (b *backendNotas) Status() control.Status { return b.st }

func newNotas(t *testing.T, st control.Status, importar bool) http.Handler {
	t.Helper()
	b := &backendNotas{st: st}
	return Handler(Options{
		Key: key, HarnessPort: 8080, CtlPort: 11080,
		Notas:        notas.New(t.TempDir()),
		PodeImportar: func() bool { return importar },
	}, b)
}

var comBanco = control.Status{Estado: control.Rodando, Motor: "postgres", Host: "h", Porta: 5432, Base: "d", Usuario: "u"}

var chaveOk = notas.Chave(notas.Banco{Motor: "postgres", Host: "h", Porta: 5432, Base: "d", Usuario: "u"})

var docOk = `{"chave":"` + chaveOk + `","dados":{"versao":1,"tabelas":{"vendas":{"nota":"oficial"}},"consultas":[]}}`

func TestNotasSemBanco(t *testing.T) {
	h := newNotas(t, control.Status{}, false)
	w := do(h, "GET", "/notas", "", nil)
	if w.Code != 409 || erro(w) != "Nenhum banco conectado." {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestNotasGravarELer(t *testing.T) {
	h := newNotas(t, comBanco, false)
	if w := do(h, "POST", "/notas", docOk, nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	w := do(h, "GET", "/notas", "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"motor":"postgres"`) || !strings.Contains(w.Body.String(), `"chave":"`+chaveOk+`"`) || !strings.Contains(w.Body.String(), `"nota":"oficial"`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestNotasInvalido(t *testing.T) {
	h := newNotas(t, comBanco, false)
	if w := do(h, "POST", "/notas", `{"chave":"`+chaveOk+`","dados":[]}`, nil); w.Code != 400 {
		t.Errorf("dados inválidos: %d %s", w.Code, w.Body)
	}
	grande := `{"chave":"` + chaveOk + `","dados":{"versao":1,"tabelas":{},"consultas":[],"x":"` + strings.Repeat("a", notas.MaxDados) + `"}}`
	if w := do(h, "POST", "/notas", grande, nil); w.Code != 413 {
		t.Errorf("grande: %d %s", w.Code, w.Body)
	}
	if w := do(newNotas(t, control.Status{}, false), "POST", "/notas", docOk, nil); w.Code != 409 {
		t.Errorf("sem banco: %d", w.Code)
	}
}

func TestImportarNoPendrive(t *testing.T) {
	h := newNotas(t, comBanco, false)
	w := do(h, "POST", "/notas/importar", docOk, nil)
	if w.Code != 403 || erro(w) != "Importar só na versão instalada." {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if g := do(h, "GET", "/notas", "", nil); strings.Contains(g.Body.String(), "oficial") {
		t.Errorf("gravou no pendrive: %s", g.Body)
	}
}

func TestImportarInstalada(t *testing.T) {
	h := newNotas(t, comBanco, true)
	if w := do(h, "POST", "/notas/importar", docOk, nil); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if g := do(h, "GET", "/notas", "", nil); !strings.Contains(g.Body.String(), `"nota":"oficial"`) {
		t.Errorf("não devolveu o importado: %s", g.Body)
	}
}

func TestNotasDesligadas(t *testing.T) {
	h, _ := newH()
	if w := do(h, "GET", "/notas", "", nil); w.Code != 404 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestNotasChave(t *testing.T) {
	outro := notas.Chave(notas.Banco{Motor: "postgres", Host: "outro", Porta: 5432, Base: "d"})
	semChave := `{"dados":{"versao":1,"tabelas":{"vendas":{"nota":"oficial"}},"consultas":[]}}`
	deOutro := `{"chave":"` + outro + `","dados":{"versao":1,"tabelas":{"vendas":{"nota":"oficial"}},"consultas":[]}}`
	for _, rota := range []string{"/notas", "/notas/importar"} {
		h := newNotas(t, comBanco, true)
		if w := do(h, "POST", rota, deOutro, nil); w.Code != 409 || erro(w) != "O banco conectado mudou. Abra a tela de novo." {
			t.Errorf("%s chave de outro banco: %d %s", rota, w.Code, w.Body)
		}
		if w := do(h, "POST", rota, semChave, nil); w.Code != 400 {
			t.Errorf("%s sem chave: %d %s", rota, w.Code, w.Body)
		}
		if g := do(h, "GET", "/notas", "", nil); strings.Contains(g.Body.String(), "oficial") {
			t.Errorf("%s gravou sem a chave certa: %s", rota, g.Body)
		}
		if w := do(h, "POST", rota, docOk, nil); w.Code != 200 {
			t.Errorf("%s chave certa: %d %s", rota, w.Code, w.Body)
		}
		if g := do(h, "GET", "/notas", "", nil); !strings.Contains(g.Body.String(), "oficial") {
			t.Errorf("%s não gravou com a chave certa: %s", rota, g.Body)
		}
	}
}

func TestNotasArquivoCorrompido(t *testing.T) {
	dir := t.TempDir()
	b := &backendNotas{st: comBanco}
	h := Handler(Options{Key: key, HarnessPort: 8080, CtlPort: 11080, Notas: notas.New(dir)}, b)
	arq := filepath.Join(dir, "notas", chaveOk+".json")
	if err := os.MkdirAll(filepath.Dir(arq), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(arq, []byte(`{"versao":1,`), 0o600); err != nil {
		t.Fatal(err)
	}
	w := do(h, "GET", "/notas", "", nil)
	if w.Code != 500 || erro(w) != "O arquivo de notas deste banco está corrompido; nada foi apagado." {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func newEntendimento(t *testing.T, st control.Status, ligado bool) http.Handler {
	t.Helper()
	o := Options{Key: key, HarnessPort: 8080, CtlPort: 11080}
	if ligado {
		o.Entendimento = notas.NewEntendimento(t.TempDir())
	}
	return Handler(o, &backendNotas{st: st})
}

func TestEntendimentoRotas(t *testing.T) {
	h := newEntendimento(t, comBanco, true)
	w := do(h, "GET", "/entendimento", "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"chave":"`+chaveOk+`"`) || !strings.Contains(w.Body.String(), `"tabelas":{}`) {
		t.Fatalf("GET: %d %s", w.Code, w.Body)
	}
	corpo := func(chave string) string {
		return `{"chave":"` + chave + `","dados":{"versao":1,"tabelas":{"vendas":{"papel":"movimento"}}}}`
	}
	if w := do(h, "POST", "/entendimento", corpo(chaveOk), nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("POST: %d %s", w.Code, w.Body)
	}
	if w := do(h, "POST", "/entendimento", corpo(strings.Repeat("0", 32)), nil); w.Code != 409 || erro(w) != "O banco conectado mudou. Abra a tela de novo." {
		t.Errorf("outra chave: %d %s", w.Code, w.Body)
	}
	if w := do(h, "POST", "/entendimento", `{"dados":{"versao":1,"tabelas":{}}}`, nil); w.Code != 400 {
		t.Errorf("sem chave: %d %s", w.Code, w.Body)
	}
	if w := do(h, "POST", "/entendimento", `{"chave":"`+chaveOk+`","dados":[]}`, nil); w.Code != 400 {
		t.Errorf("inválido: %d %s", w.Code, w.Body)
	}
	grande := `{"chave":"` + chaveOk + `","dados":{"versao":1,"tabelas":{},"x":"` + strings.Repeat("a", notas.MaxEntendimento) + `"}}`
	if w := do(h, "POST", "/entendimento", grande, nil); w.Code != 413 {
		t.Errorf("grande: %d", w.Code)
	}
	if g := do(h, "GET", "/entendimento", "", nil); !strings.Contains(g.Body.String(), `"papel":"movimento"`) || strings.Contains(g.Body.String(), `"x":"a`) {
		t.Errorf("estado final: %s", g.Body)
	}
	if w := do(newEntendimento(t, control.Status{}, true), "GET", "/entendimento", "", nil); w.Code != 409 {
		t.Errorf("sem banco GET: %d", w.Code)
	}
	if w := do(newEntendimento(t, control.Status{}, true), "POST", "/entendimento", corpo(chaveOk), nil); w.Code != 409 {
		t.Errorf("sem banco POST: %d", w.Code)
	}
	if w := do(h, "DELETE", "/entendimento", "", nil); w.Code != 405 {
		t.Errorf("DELETE: %d", w.Code)
	}
	if w := do(h, "POST", "/entendimento/importar", corpo(chaveOk), nil); w.Code != 404 {
		t.Errorf("importar: %d", w.Code)
	}
}

func TestEntendimentoDesligado(t *testing.T) {
	h := newEntendimento(t, comBanco, false)
	if w := do(h, "GET", "/entendimento", "", nil); w.Code != 404 {
		t.Errorf("%d", w.Code)
	}
}

// Arquivo do entendimento corrompido: o GET avisa (500) e a gravação seguinte sobrescreve com o documento novo.
func TestEntendimentoArquivoCorrompido(t *testing.T) {
	dir := t.TempDir()
	h := Handler(Options{Key: key, HarnessPort: 8080, CtlPort: 11080, Entendimento: notas.NewEntendimento(dir)}, &backendNotas{st: comBanco})
	arq := filepath.Join(dir, "entendimento", chaveOk+".json")
	if err := os.MkdirAll(filepath.Dir(arq), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(arq, []byte(`{"versao":1,`), 0o600); err != nil {
		t.Fatal(err)
	}
	w := do(h, "GET", "/entendimento", "", nil)
	if w.Code != 500 || erro(w) != "O arquivo do entendimento deste banco está corrompido; nada foi apagado." {
		t.Fatalf("GET corrompido: %d %s", w.Code, w.Body)
	}
	novo := `{"chave":"` + chaveOk + `","dados":{"versao":1,"completo":true,"tabelas":{"vendas":{"papel":"movimento","campos":[]}}}}`
	if w := do(h, "POST", "/entendimento", novo, nil); w.Code != 200 {
		t.Fatalf("POST por cima do corrompido: %d %s", w.Code, w.Body)
	}
	if g := do(h, "GET", "/entendimento", "", nil); g.Code != 200 || !strings.Contains(g.Body.String(), `"papel":"movimento"`) {
		t.Errorf("GET depois de sobrescrever: %d %s", g.Code, g.Body)
	}
}

// Falha de leitura que não é corrupção (o caminho do arquivo é uma pasta): frase inteira, sem emenda.
func TestLerDocErroDeLeitura(t *testing.T) {
	dir := t.TempDir()
	h := Handler(Options{Key: key, HarnessPort: 8080, CtlPort: 11080, Notas: notas.New(dir), Entendimento: notas.NewEntendimento(dir)}, &backendNotas{st: comBanco})
	for _, c := range []struct{ sub, rota, msg string }{
		{"notas", "/notas", "Não consegui ler as notas."},
		{"entendimento", "/entendimento", "Não consegui ler o entendimento."},
	} {
		if err := os.MkdirAll(filepath.Join(dir, c.sub, chaveOk+".json"), 0o700); err != nil {
			t.Fatal(err)
		}
		if w := do(h, "GET", c.rota, "", nil); w.Code != 500 || erro(w) != c.msg {
			t.Errorf("%s: %d %s", c.rota, w.Code, w.Body)
		}
	}
}
