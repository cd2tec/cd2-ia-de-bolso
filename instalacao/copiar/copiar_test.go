package copiar

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func escrever(t *testing.T, raiz, rel, conteudo string, modo os.FileMode) {
	t.Helper()
	p := filepath.Join(raiz, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(conteudo), modo); err != nil {
		t.Fatal(err)
	}
}

// penDriveFalso monta um pen drive com os arquivos de todas as plataformas e
// o CHECKSUMS.sha256 dos binários grandes.
func penDriveFalso(t *testing.T, versao string) string {
	t.Helper()
	raiz := t.TempDir()
	arq := map[string]string{
		"VERSION":                               versao,
		"LICENSE":                               "licenca",
		"README.md":                             "leia",
		"INICIAR-CD2-Windows.bat":               "bat",
		"web/index.html":                        "<html>",
		"web/js/app.js":                         "js",
		"web/._index.html":                      "lixo",
		"web/.DS_Store":                         "lixo",
		"core/motor.txt":                        "motor",
		"bin/llamafile":                         "llamafile",
		"bin/toolbox/windows-amd64/toolbox.exe": "tbw",
		"bin/toolbox/darwin-arm64/toolbox":      "tba",
		"bin/toolbox/darwin-amd64/toolbox":      "tbi",
		"bin/ctl/windows-amd64/cd2-ctl.exe":     "ctlw",
		"bin/ctl/darwin-arm64/cd2-ctl":          "ctla",
		"bin/ctl/darwin-amd64/cd2-ctl":          "ctli",
		"models/modelo.gguf":                    "modelo-grande",
		"IA de Bolso.exe":                       "exe",
		"IA de Bolso.app/Contents/Info.plist":   "plist",
		"IA de Bolso.app/Contents/MacOS/bolso":  "binario",
		"IA de Bolso.app/Contents/._Info.plist": "lixo",
	}
	var linhas []string
	for rel, c := range arq {
		modo := os.FileMode(0o644)
		if rel == "bin/llamafile" || strings.HasSuffix(rel, "MacOS/bolso") {
			modo = 0o755
		}
		escrever(t, raiz, rel, c, modo)
		if strings.HasPrefix(rel, "bin/") || strings.HasPrefix(rel, "models/") {
			s := sha256.Sum256([]byte(c))
			linhas = append(linhas, hex.EncodeToString(s[:])+"  "+rel)
		}
	}
	sort.Strings(linhas)
	escrever(t, raiz, "CHECKSUMS.sha256", strings.Join(linhas, "\n")+"\n", 0o644)
	return raiz
}

func destinos(itens []Item) []string {
	var r []string
	for _, it := range itens {
		r = append(r, it.Destino)
	}
	sort.Strings(r)
	return r
}

func TestPlanejarSoAPlataforma(t *testing.T) {
	raiz := penDriveFalso(t, "1.0.0")
	itens, total, err := Planejar(raiz, "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(destinos(itens), "|")
	for _, quer := range []string{"VERSION", "LICENSE", "web/index.html", "web/js/app.js", "core/motor.txt",
		"bin/llamafile", "models/modelo.gguf", "bin/toolbox/darwin-arm64/toolbox", "bin/ctl/darwin-arm64/cd2-ctl",
		"IA de Bolso.app/Contents/Info.plist", "IA de Bolso.app/Contents/MacOS/bolso"} {
		if !strings.Contains(got, quer) {
			t.Errorf("faltou %s em %s", quer, got)
		}
	}
	for _, nao := range []string{"windows", "darwin-amd64", ".exe", "._", ".DS_Store", "README", "CHECKSUMS", ".bat"} {
		if strings.Contains(got, nao) {
			t.Errorf("não devia ter %q em %s", nao, got)
		}
	}
	var soma int64
	for _, it := range itens {
		soma += it.Tamanho
		if strings.HasPrefix(it.Destino, "bin/") && it.SHA256 == "" {
			t.Errorf("%s sem sha", it.Destino)
		}
		if strings.HasPrefix(it.Destino, "web/") && it.SHA256 != "" {
			t.Errorf("%s não devia ter sha", it.Destino)
		}
	}
	if soma != total {
		t.Errorf("total %d != soma %d", total, soma)
	}
}

func TestPlanejarWindows(t *testing.T) {
	raiz := penDriveFalso(t, "1.0.0")
	itens, _, err := Planejar(raiz, "windows", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(destinos(itens), "|")
	if !strings.Contains(got, "IA de Bolso.exe") || !strings.Contains(got, "bin/toolbox/windows-amd64/toolbox.exe") ||
		!strings.Contains(got, "bin/ctl/windows-amd64/cd2-ctl.exe") || strings.Contains(got, ".app") || strings.Contains(got, "darwin") {
		t.Errorf("plano do windows errado: %s", got)
	}
}

// No Windows o lançador procura bin\llamafile.exe na instalação. O pen drive
// traz os dois nomes com os mesmos bytes; o CHECKSUMS só tem bin/llamafile.
func TestPlanejarWindowsMotorExe(t *testing.T) {
	for _, comExe := range []bool{true, false} {
		raiz := penDriveFalso(t, "1.0.0")
		if comExe {
			escrever(t, raiz, "bin/llamafile.exe", "llamafile", 0o644)
		}
		itens, _, err := Planejar(raiz, "windows", "amd64")
		if err != nil {
			t.Fatal(err)
		}
		s := sha256.Sum256([]byte("llamafile"))
		var achou bool
		for _, it := range itens {
			if it.Destino == "bin/llamafile" {
				t.Errorf("comExe=%v: o Windows não devia planejar bin/llamafile", comExe)
			}
			if it.Destino != "bin/llamafile.exe" {
				continue
			}
			achou = true
			// A origem é sempre bin/llamafile, o arquivo que o CHECKSUMS confere:
			// o .exe pode ter ficado de uma versão anterior do motor.
			origem := "llamafile"
			if filepath.Base(it.Origem) != origem {
				t.Errorf("comExe=%v: origem %s, queria %s", comExe, it.Origem, origem)
			}
			if it.SHA256 != hex.EncodeToString(s[:]) {
				t.Errorf("comExe=%v: sha %q", comExe, it.SHA256)
			}
		}
		if !achou {
			t.Errorf("comExe=%v: faltou bin/llamafile.exe no plano: %v", comExe, destinos(itens))
		}
	}
}

func TestPlanejarFaltaPeca(t *testing.T) {
	for _, rel := range []string{"models/modelo.gguf", "bin/llamafile", "IA de Bolso.app", "bin/toolbox/darwin-amd64", "bin/ctl/darwin-amd64", "web", "core"} {
		raiz := penDriveFalso(t, "1.0.0")
		if err := os.RemoveAll(filepath.Join(raiz, filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
		if _, _, err := Planejar(raiz, "darwin", "amd64"); err == nil {
			t.Errorf("sem %s devia dar erro", rel)
		}
	}
}

func TestCopiaConfereChecksum(t *testing.T) {
	raiz := penDriveFalso(t, "1.0.0")
	itens, _, err := Planejar(raiz, "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	escrever(t, raiz, "models/modelo.gguf", "modelo-adulterado", 0o644)
	dest := filepath.Join(t.TempDir(), "IA")
	err = Copiar(context.Background(), itens, dest, nil)
	if !errors.Is(err, ErrCopiaRuim) {
		t.Fatalf("esperava ErrCopiaRuim, veio %v", err)
	}
	if err.Error() != "Não consegui copiar direito. Tente de novo." {
		t.Errorf("mensagem: %q", err.Error())
	}
	if _, e := os.Stat(dest); !os.IsNotExist(e) {
		t.Error("destino não devia existir")
	}
	if _, e := os.Stat(dest + ".novo"); !os.IsNotExist(e) {
		t.Error(".novo devia ter sido apagado")
	}
}

func TestCopiaQueFalhaNaoEstragaAnterior(t *testing.T) {
	raiz := penDriveFalso(t, "1.0.0")
	itens, _, _ := Planejar(raiz, "darwin", "arm64")
	dest := filepath.Join(t.TempDir(), "IA")
	escrever(t, dest, "VERSION", "0.14.0", 0o644)
	ctx, cancela := context.WithCancel(context.Background())
	n := 0
	err := Copiar(ctx, itens, dest, func(f, tot int64) {
		n++
		if n == 3 {
			cancela()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("esperava cancelamento, veio %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(dest, "VERSION"))
	if string(b) != "0.14.0" {
		t.Errorf("VERSION = %q", b)
	}
	if _, e := os.Stat(dest + ".novo"); !os.IsNotExist(e) {
		t.Error(".novo devia ter sido apagado")
	}
}

func TestCopiaSubstituiAnterior(t *testing.T) {
	raiz := penDriveFalso(t, "1.0.0")
	itens, _, _ := Planejar(raiz, "darwin", "arm64")
	dest := filepath.Join(t.TempDir(), "IA")
	escrever(t, dest, "VERSION", "0.14.0", 0o644)
	escrever(t, dest, "sobra-antiga.txt", "x", 0o644)
	// restos de uma queda anterior
	escrever(t, dest+".novo", "lixo", "x", 0o644)
	if err := Copiar(context.Background(), itens, dest, nil); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dest, "VERSION"))
	if string(b) != "1.0.0" {
		t.Errorf("VERSION = %q", b)
	}
	for _, p := range []string{dest + ".velho", dest + ".novo", filepath.Join(dest, "sobra-antiga.txt"), filepath.Join(dest, "web", "._index.html")} {
		if _, e := os.Stat(p); !os.IsNotExist(e) {
			t.Errorf("%s não devia existir", p)
		}
	}
}

func TestCopiaPreservaExecutavel(t *testing.T) {
	raiz := penDriveFalso(t, "1.0.0")
	itens, _, _ := Planejar(raiz, "darwin", "arm64")
	dest := filepath.Join(t.TempDir(), "IA")
	if err := Copiar(context.Background(), itens, dest, nil); err != nil {
		t.Fatal(err)
	}
	for rel, quer := range map[string]bool{"bin/llamafile": true, "IA de Bolso.app/Contents/MacOS/bolso": true, "VERSION": false} {
		fi, err := os.Stat(filepath.Join(dest, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if (fi.Mode()&0o111 != 0) != quer {
			t.Errorf("%s executável = %v, quer %v", rel, !quer, quer)
		}
	}
}

func TestProgressoChegaAoTotal(t *testing.T) {
	raiz := penDriveFalso(t, "1.0.0")
	itens, total, _ := Planejar(raiz, "darwin", "arm64")
	var ultF, ultT int64
	err := Copiar(context.Background(), itens, filepath.Join(t.TempDir(), "IA"), func(f, tot int64) { ultF, ultT = f, tot })
	if err != nil {
		t.Fatal(err)
	}
	if ultF != total || ultT != total {
		t.Errorf("último progresso %d/%d, total %d", ultF, ultT, total)
	}
}

func TestSemEspacoNaoCriaNada(t *testing.T) {
	raiz := penDriveFalso(t, "1.0.0")
	itens, _, _ := Planejar(raiz, "darwin", "arm64")
	velho := espacoLivre
	espacoLivre = func(string) (int64, error) { return 1, nil }
	defer func() { espacoLivre = velho }()
	pai := t.TempDir()
	err := Copiar(context.Background(), itens, filepath.Join(pai, "IA"), nil)
	if !errors.Is(err, ErrSemEspaco) {
		t.Fatalf("esperava ErrSemEspaco, veio %v", err)
	}
	if err.Error() != "Falta espaço no computador. Libere uns 3 GB e tente de novo." {
		t.Errorf("mensagem: %q", err.Error())
	}
	if ents, _ := os.ReadDir(pai); len(ents) != 0 {
		t.Errorf("criou %d entradas", len(ents))
	}
}

func TestEspacoLivreReal(t *testing.T) {
	n, err := EspacoLivre(t.TempDir())
	if err != nil || n <= 0 {
		t.Errorf("EspacoLivre = %d, %v", n, err)
	}
}

func TestRenomearFalhaDaErrEmUso(t *testing.T) {
	raiz := penDriveFalso(t, "1.0.0")
	itens, _, _ := Planejar(raiz, "darwin", "arm64")
	dest := filepath.Join(t.TempDir(), "IA")
	escrever(t, dest, "VERSION", "0.14.0", 0o644)
	velhoR, velhoE := renomear, esperaRenomear
	chamadas := 0
	renomear = func(a, b string) error { chamadas++; return errors.New("Access is denied.") }
	esperaRenomear = 0
	defer func() { renomear, esperaRenomear = velhoR, velhoE }()
	err := Copiar(context.Background(), itens, dest, nil)
	if !errors.Is(err, ErrEmUso) {
		t.Fatalf("esperava ErrEmUso, veio %v", err)
	}
	if chamadas != 3 {
		t.Errorf("tentativas = %d", chamadas)
	}
	b, _ := os.ReadFile(filepath.Join(dest, "VERSION"))
	if string(b) != "0.14.0" {
		t.Errorf("VERSION = %q", b)
	}
	if _, e := os.Stat(dest + ".novo"); !os.IsNotExist(e) {
		t.Error(".novo devia ter sido apagado")
	}
}

func TestPlanejarExigeChecksumEmBin(t *testing.T) {
	raiz := penDriveFalso(t, "1.0.0")
	b, _ := os.ReadFile(filepath.Join(raiz, "CHECKSUMS.sha256"))
	var ls []string
	for _, l := range strings.Split(string(b), "\n") {
		if !strings.HasSuffix(l, "  bin/llamafile") {
			ls = append(ls, l)
		}
	}
	escrever(t, raiz, "CHECKSUMS.sha256", strings.Join(ls, "\n"), 0o644)
	_, _, err := Planejar(raiz, "darwin", "arm64")
	if err == nil || !strings.Contains(err.Error(), "bin/llamafile") {
		t.Fatalf("esperava erro citando bin/llamafile, veio %v", err)
	}
}

func TestRestauraVelhoOrfao(t *testing.T) {
	raiz := penDriveFalso(t, "1.0.0")
	itens, _, _ := Planejar(raiz, "darwin", "arm64")
	dest := filepath.Join(t.TempDir(), "IA")
	escrever(t, dest+".velho", "VERSION", "0.14.0", 0o644)
	ctx, cancela := context.WithCancel(context.Background())
	cancela()
	if err := Copiar(ctx, itens, dest, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("esperava cancelamento, veio %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(dest, "VERSION"))
	if string(b) != "0.14.0" {
		t.Errorf("VERSION = %q, .velho não foi restaurado", b)
	}
}

func TestDestinoNaoLocalRecusado(t *testing.T) {
	err := Copiar(context.Background(), []Item{{Origem: "x", Destino: "../fora"}}, filepath.Join(t.TempDir(), "IA"), nil)
	if err == nil {
		t.Error("devia recusar destino fora da pasta")
	}
}

// CopiarAvisando conta qual item começou, na ordem do plano, antes dos bytes dele.
func TestCopiarAvisandoDizOItemAtual(t *testing.T) {
	raiz := penDriveFalso(t, "1.0.0")
	itens, _, _ := Planejar(raiz, "darwin", "arm64")
	var vistos []string
	var feitosNoInicio []int64
	var feitos int64
	err := CopiarAvisando(context.Background(), itens, filepath.Join(t.TempDir(), "IA"),
		func(f, _ int64) { feitos = f },
		func(destino string) {
			vistos = append(vistos, destino)
			feitosNoInicio = append(feitosNoInicio, feitos)
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(vistos) != len(itens) {
		t.Fatalf("avisou %d itens de %d", len(vistos), len(itens))
	}
	var antes int64
	for i, it := range itens {
		if vistos[i] != it.Destino || feitosNoInicio[i] != antes {
			t.Errorf("item %d: %s com %d feitos, queria %s com %d", i, vistos[i], feitosNoInicio[i], it.Destino, antes)
		}
		antes += it.Tamanho
	}
}
