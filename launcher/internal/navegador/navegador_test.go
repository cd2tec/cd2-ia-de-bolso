package navegador

import (
	"reflect"
	"testing"
)

const (
	pf86   = `C:\PF86`
	pf     = `C:\PF`
	local  = `C:\Users\x\AppData\Local`
	edge86 = pf86 + `\Microsoft\Edge\Application\msedge.exe`
	chrome = pf + `\Google\Chrome\Application\chrome.exe`
	chLoc  = local + `\Google\Chrome\Application\chrome.exe`
)

func ambiente(k string) string {
	return map[string]string{"ProgramFiles(x86)": pf86, "ProgramFiles": pf, "LOCALAPPDATA": local}[k]
}

func so(caminhos ...string) func(string) bool {
	return func(p string) bool {
		for _, c := range caminhos {
			if c == p {
				return true
			}
		}
		return false
	}
}

func TestURL(t *testing.T) {
	if got := URL(8080, "0.14.0", "ab"); got != "http://127.0.0.1:8080/?v=0.14.0#k=ab" {
		t.Fatalf("URL = %q", got)
	}
}

func TestWindowsPrefereEdge(t *testing.T) {
	nome, args := Comando("windows", "U", so(edge86, chrome), ambiente, "")
	if nome != edge86 || !reflect.DeepEqual(args, []string{"--app=U", "--start-maximized"}) {
		t.Fatalf("got %q %q", nome, args)
	}
}

func TestWindowsSoChromeLocal(t *testing.T) {
	nome, _ := Comando("windows", "U", so(chLoc), ambiente, "")
	if nome != chLoc {
		t.Fatalf("nome = %q", nome)
	}
}

func TestWindowsPulaVariavelVazia(t *testing.T) {
	// Sem ProgramFiles(x86), um caminho "\Microsoft\..." sem raiz não pode valer.
	env := func(k string) string {
		if k == "ProgramFiles" {
			return pf
		}
		return ""
	}
	existe := func(p string) bool { return p == `\Microsoft\Edge\Application\msedge.exe` || p == chrome }
	nome, _ := Comando("windows", "U", existe, env, "")
	if nome != chrome {
		t.Fatalf("nome = %q", nome)
	}
}

func TestMacChromeComTamanho(t *testing.T) {
	nome, args := Comando("darwin", "U", so("/Applications/Google Chrome.app"), nil, "1440,875")
	want := []string{"-na", "Google Chrome", "--args", "--app=U", "--start-maximized", "--window-position=0,0", "--window-size=1440,875"}
	if nome != "open" || !reflect.DeepEqual(args, want) {
		t.Fatalf("got %q %q", nome, args)
	}
}

func TestMacSemTamanho(t *testing.T) {
	_, args := Comando("darwin", "U", so("/Applications/Microsoft Edge.app"), nil, "")
	want := []string{"-na", "Microsoft Edge", "--args", "--app=U", "--start-maximized"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %q", args)
	}
}

func TestSemNavegadorConhecido(t *testing.T) {
	nenhum := so()
	if n, a := Comando("darwin", "U", nenhum, nil, ""); n != "open" || !reflect.DeepEqual(a, []string{"U"}) {
		t.Fatalf("darwin: %q %q", n, a)
	}
	if n, a := Comando("windows", "U", nenhum, ambiente, ""); n != "rundll32" || !reflect.DeepEqual(a, []string{"url.dll,FileProtocolHandler", "U"}) {
		t.Fatalf("windows: %q %q", n, a)
	}
}

func TestMacPrioridade(t *testing.T) {
	_, a := Comando("darwin", "U", so("/Applications/Microsoft Edge.app", "/Applications/Google Chrome.app"), nil, "")
	if a[1] != "Google Chrome" {
		t.Fatalf("esperava Chrome, veio %q", a[1])
	}
	_, a = Comando("darwin", "U", so("/Applications/Chromium.app", "/Applications/Brave Browser.app"), nil, "")
	if a[1] != "Brave Browser" {
		t.Fatalf("esperava Brave, veio %q", a[1])
	}
}

func TestComInstalar(t *testing.T) {
	casos := map[string]string{
		"http://127.0.0.1:8080/?v=1#k=abc": "http://127.0.0.1:8080/?v=1&instalar=1#k=abc",
		"http://127.0.0.1:8080/?v=1":       "http://127.0.0.1:8080/?v=1&instalar=1",
		"http://127.0.0.1:8080/#k=abc":     "http://127.0.0.1:8080/?instalar=1#k=abc",
		"http://127.0.0.1:8080/":           "http://127.0.0.1:8080/?instalar=1",
	}
	for url, quer := range casos {
		if got := ComInstalar(url); got != quer {
			t.Errorf("ComInstalar(%q) = %q, quer %q", url, got, quer)
		}
	}
}
