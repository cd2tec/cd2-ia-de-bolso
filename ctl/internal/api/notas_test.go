package api

import (
	"net/http"
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

const docOk = `{"dados":{"versao":1,"tabelas":{"vendas":{"nota":"oficial"}},"consultas":[]}}`

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
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"motor":"postgres"`) || !strings.Contains(w.Body.String(), `"nota":"oficial"`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestNotasInvalido(t *testing.T) {
	h := newNotas(t, comBanco, false)
	if w := do(h, "POST", "/notas", `{"dados":[]}`, nil); w.Code != 400 {
		t.Errorf("dados inválidos: %d %s", w.Code, w.Body)
	}
	grande := `{"dados":{"versao":1,"tabelas":{},"consultas":[],"x":"` + strings.Repeat("a", notas.MaxDados) + `"}}`
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
