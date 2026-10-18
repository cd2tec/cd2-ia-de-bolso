package api

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/inst"
)

type fakeInst struct {
	info     inst.Info
	chaveErr error
	instalar []bool
	instErr  error
	prog     inst.Progresso
	abrirErr error
	abriu    int
	gravadas []string
}

func (f *fakeInst) Info() inst.Info { return f.info }
func (f *fakeInst) GravarChave(k string) error {
	f.gravadas = append(f.gravadas, k)
	return f.chaveErr
}
func (f *fakeInst) Instalar(a, i bool) error {
	f.instalar = append(f.instalar, a, i)
	return f.instErr
}
func (f *fakeInst) Progresso() inst.Progresso { return f.prog }
func (f *fakeInst) Abrir() error              { f.abriu++; return f.abrirErr }

func newHI() (http.Handler, *fakeInst) {
	fi := &fakeInst{info: inst.Info{Origem: "pendrive", Codigo: "ABCD-EFGH-JKLM"}}
	return Handler(Options{Key: key, HarnessPort: 8080, CtlPort: 11080, Instalacao: fi}, &fakeBackend{}), fi
}

func TestInstalacaoExigeChaveDaSessao(t *testing.T) {
	h, _ := newHI()
	for _, c := range [][2]string{{"GET", "/instalacao"}, {"POST", "/instalacao/chave"}, {"POST", "/instalacao"}, {"GET", "/instalacao/progresso"}, {"POST", "/instalacao/abrir"}} {
		w := do(h, c[0], c[1], `{}`, func(r *http.Request) { r.Header.Del("Authorization") })
		if w.Code != 401 {
			t.Errorf("%v: %d", c, w.Code)
		}
	}
}

func TestInstalacaoStatus(t *testing.T) {
	h, _ := newHI()
	w := do(h, "GET", "/instalacao", "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"origem":"pendrive"`) || !strings.Contains(w.Body.String(), `"codigo":"ABCD-EFGH-JKLM"`) ||
		!strings.Contains(w.Body.String(), `"temChave":false`) || !strings.Contains(w.Body.String(), `"instalada":false`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestChaveCertaGrava(t *testing.T) {
	h, fi := newHI()
	w := do(h, "POST", "/instalacao/chave", `{"chave":"abc"}`, nil)
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"ok":true}` || len(fi.gravadas) != 1 || fi.gravadas[0] != "abc" {
		t.Fatalf("%d %s %v", w.Code, w.Body, fi.gravadas)
	}
}

func TestChaveErradaDa400(t *testing.T) {
	h, fi := newHI()
	fi.chaveErr = inst.ErrChaveNaoVale
	w := do(h, "POST", "/instalacao/chave", `{"chave":"x"}`, nil)
	if w.Code != 400 || erro(w) != "Esta chave não é deste computador." {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestInstalarComecaEm202(t *testing.T) {
	h, fi := newHI()
	w := do(h, "POST", "/instalacao", `{"areaDeTrabalho":true,"inicio":false}`, nil)
	if w.Code != 202 || strings.TrimSpace(w.Body.String()) != `{"ok":true}` || len(fi.instalar) != 2 || !fi.instalar[0] || fi.instalar[1] {
		t.Fatalf("%d %s %v", w.Code, w.Body, fi.instalar)
	}
}

func TestInstalarExigeCorpoValido(t *testing.T) {
	h, fi := newHI()
	for _, body := range []string{``, `{"areaDeTrabalho":true,"x":1}`, `nao`} {
		if w := do(h, "POST", "/instalacao", body, nil); w.Code != 400 {
			t.Errorf("%q: %d", body, w.Code)
		}
	}
	if len(fi.instalar) != 0 {
		t.Error("não devia ter começado")
	}
}

func TestInstalarDuasVezesDa409(t *testing.T) {
	h, fi := newHI()
	fi.instErr = inst.ErrOcupada
	if w := do(h, "POST", "/instalacao", `{"areaDeTrabalho":false,"inicio":false}`, nil); w.Code != 409 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestInstalarSemChaveDa403(t *testing.T) {
	h, fi := newHI()
	fi.instErr = inst.ErrSemChave
	w := do(h, "POST", "/instalacao", `{"areaDeTrabalho":false,"inicio":false}`, nil)
	if w.Code != 403 || erro(w) != "Falta a chave deste computador." {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestInstalarErroInesperadoDa500(t *testing.T) {
	h, fi := newHI()
	fi.instErr = errors.New("x")
	if w := do(h, "POST", "/instalacao", `{"areaDeTrabalho":false,"inicio":false}`, nil); w.Code != 500 {
		t.Fatalf("%d", w.Code)
	}
}

func TestProgresso(t *testing.T) {
	h, fi := newHI()
	fi.prog = inst.Progresso{Fase: "pronta", Feitos: 5, Total: 5, Aviso: "aviso"}
	w := do(h, "GET", "/instalacao/progresso", "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"fase":"pronta"`) || !strings.Contains(w.Body.String(), `"aviso":"aviso"`) || strings.Contains(w.Body.String(), `"erro"`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestAbrir(t *testing.T) {
	h, fi := newHI()
	if w := do(h, "POST", "/instalacao/abrir", "", nil); w.Code != 202 || fi.abriu != 1 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	fi.abrirErr = inst.ErrNaoInstalada
	w := do(h, "POST", "/instalacao/abrir", "", nil)
	if w.Code != 409 || erro(w) != "A IA ainda não foi instalada." {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestMetodoErradoNasRotasDaInstalacao(t *testing.T) {
	h, _ := newHI()
	for _, c := range [][2]string{{"POST", "/instalacao/progresso"}, {"GET", "/instalacao/abrir"}, {"GET", "/instalacao/chave"}, {"DELETE", "/instalacao"}} {
		if w := do(h, c[0], c[1], "{}", nil); w.Code != 405 {
			t.Errorf("%v: %d", c, w.Code)
		}
	}
}

func TestPreflightDasRotasNovas(t *testing.T) {
	h, _ := newHI()
	w := do(h, "OPTIONS", "/instalacao", "", func(r *http.Request) { r.Header.Del("Authorization") })
	if w.Code != 204 || !strings.Contains(w.Header().Get("Access-Control-Allow-Methods"), "POST") {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
}

func TestSemBackendDa404(t *testing.T) {
	h, _ := newH()
	for _, p := range []string{"/instalacao", "/instalacao/chave", "/instalacao/progresso", "/instalacao/abrir"} {
		if w := do(h, "GET", p, "", nil); w.Code != 404 {
			t.Errorf("%s: %d", p, w.Code)
		}
	}
	if w := do(h, "GET", "/status", "", nil); w.Code != 200 {
		t.Errorf("/status: %d", w.Code)
	}
}

func TestForaDoPenDriveDa403(t *testing.T) {
	h, fi := newHI()
	fi.chaveErr, fi.instErr = inst.ErrSoPendrive, inst.ErrSoPendrive
	for _, c := range [][2]string{{"/instalacao/chave", `{"chave":"x"}`}, {"/instalacao", `{"areaDeTrabalho":false,"inicio":false}`}} {
		w := do(h, "POST", c[0], c[1], nil)
		if w.Code != 403 || erro(w) != "A instalação é feita pelo pen drive." {
			t.Errorf("%s: %d %s", c[0], w.Code, w.Body)
		}
	}
}
