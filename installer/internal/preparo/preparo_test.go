package preparo

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/manifesto"
)

var plats = []string{"windows-amd64", "darwin-arm64", "darwin-amd64", "linux-amd64"}

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

type ambiente struct {
	srv      *httptest.Server
	m        *manifesto.Manifesto
	cache    string
	mu       sync.Mutex
	gets     map[string]int
	arquivos map[string][]byte
}

func (a *ambiente) contar(p string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.gets[p]
}

func criarZip(t *testing.T, comLancador bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(nome, conteudo string, modo os.FileMode) {
		h := &zip.FileHeader{Name: "cd2tech-iadebolso/" + nome, Method: zip.Deflate}
		h.SetMode(modo)
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(conteudo))
	}
	add("INICIAR-CD2-Windows.bat", "bat", 0o644)
	add("INICIAR-CD2-Mac.command", "mac", 0o755)
	add("iniciar-cd2-linux.sh", "linux", 0o755)
	add("README.md", "readme", 0o644)
	add("LICENSE", "lic", 0o644)
	add("VERSION", "0.13.0", 0o644)
	add("web/index.html", "<html>", 0o644)
	add("web/.DS_Store", "x", 0o644)
	add("core/server.js", "js", 0o644)
	add("core/prepare-unix.sh", "sh", 0o755)
	add("core/make-usb-unix.sh", "no", 0o755)
	add("core/make-usb-windows.ps1", "no", 0o644)
	add("core/._lixo", "no", 0o644)
	add("docs/a.md", "no", 0o644)
	add("ctl/main.go", "no", 0o644)
	add("tests/t.js", "no", 0o644)
	add("GRAVAR-PENDRIVE-Mac.command", "no", 0o755)
	add("GRAVAR-PENDRIVE-Windows.bat", "no", 0o644)
	add("gravar-pendrive-linux.sh", "no", 0o755)
	add(".DS_Store", "no", 0o644)
	add("._README.md", "no", 0o644)
	if comLancador {
		add("IA de Bolso.exe", "exe", 0o644)
		add("IA de Bolso.app/Contents/MacOS/IA de Bolso", "macho", 0o644)
		add("IA de Bolso.app/Contents/Info.plist", "plist", 0o644)
		add("IA de Bolso.app/Contents/._lixo", "no", 0o644)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func novoAmbiente(t *testing.T) *ambiente { return novoAmbienteCom(t, false) }

func novoAmbienteCom(t *testing.T, comLancador bool) *ambiente {
	t.Helper()
	old := minimos
	minimos.Modelo, minimos.Toolbox = 10, 10
	t.Cleanup(func() { minimos = old })

	a := &ambiente{cache: t.TempDir(), gets: map[string]int{}}
	zipB := criarZip(t, comLancador)
	modelo := []byte("modelo-conteudo-grande")
	motor := []byte("llamafile-conteudo")
	a.arquivos = map[string][]byte{
		"/zip":       zipB,
		"/llamafile": motor,
		"/modelo":    modelo,
	}
	var sums strings.Builder
	for _, p := range plats {
		os_, arch, _ := strings.Cut(p, "-")
		ext := ""
		if os_ == "windows" {
			ext = ".exe"
		}
		a.arquivos[fmt.Sprintf("/tb/%s/%s/toolbox%s", os_, arch, ext)] = []byte("toolbox-" + p)
		c := []byte("ctl-" + p)
		a.arquivos["/ctl/cd2-ctl-"+p+ext] = c
		fmt.Fprintf(&sums, "%s  cd2-ctl-%s%s\n", sha(c), p, ext)
	}
	a.arquivos["/ctl/SHA256SUMS"] = []byte(sums.String())
	a.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		if r.Method == http.MethodGet {
			a.gets[r.URL.Path]++
		}
		b, ok := a.arquivos[r.URL.Path]
		a.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(b))
	}))
	t.Cleanup(a.srv.Close)

	a.m = &manifesto.Manifesto{
		Versao:     "0.13.0",
		Zip:        manifesto.Arquivo{URL: a.srv.URL + "/zip", SHA256: sha(zipB), Bytes: int64(len(zipB))},
		Llamafile:  manifesto.Arquivo{URL: a.srv.URL + "/llamafile", SHA256: sha(motor), Bytes: int64(len(motor))},
		Modelo:     manifesto.Arquivo{URL: a.srv.URL + "/modelo", SHA256: sha(modelo)},
		ModeloNome: "modelo.gguf",
	}
	a.m.Toolbox.Versao = "1.14.0"
	a.m.Toolbox.Base = a.srv.URL + "/tb"
	a.m.Toolbox.Plataformas = plats
	a.m.Ctl.Versao = "0.6.0"
	a.m.Ctl.Base = a.srv.URL + "/ctl"
	return a
}

func (a *ambiente) montar(t *testing.T, semRede bool) (string, error) {
	t.Helper()
	return Montar(context.Background(), a.srv.Client(), a.m, a.cache, semRede, nil)
}

func existe(p string) bool { _, err := os.Stat(p); return err == nil }

func extensao(plat string) string {
	if strings.HasPrefix(plat, "windows") {
		return ".exe"
	}
	return ""
}

func TestMontarLayout(t *testing.T) {
	a := novoAmbiente(t)
	stage, err := a.montar(t, false)
	if err != nil {
		t.Fatal(err)
	}
	if stage != filepath.Join(a.cache, "CD2-HARNESS") {
		t.Fatalf("stage = %s", stage)
	}
	deve := []string{"INICIAR-CD2-Windows.bat", "INICIAR-CD2-Mac.command", "iniciar-cd2-linux.sh", "README.md",
		"LICENSE", "VERSION", "web/index.html", "core/server.js", "core/prepare-unix.sh", "bin/llamafile",
		"bin/llamafile.exe", "models/modelo.gguf", "CHECKSUMS.sha256"}
	for _, p := range plats {
		deve = append(deve, "bin/toolbox/"+p+"/toolbox"+extensao(p), "bin/ctl/"+p+"/cd2-ctl"+extensao(p))
	}
	for _, p := range deve {
		if !existe(filepath.Join(stage, p)) {
			t.Errorf("falta %s", p)
		}
	}
	a1, _ := os.ReadFile(filepath.Join(stage, "bin/llamafile"))
	a2, _ := os.ReadFile(filepath.Join(stage, "bin/llamafile.exe"))
	if !bytes.Equal(a1, a2) || len(a1) == 0 {
		t.Error("llamafile.exe diferente de llamafile")
	}
	nao := []string{"core/make-usb-unix.sh", "core/make-usb-windows.ps1", "docs", "ctl", "tests",
		"GRAVAR-PENDRIVE-Mac.command", "GRAVAR-PENDRIVE-Windows.bat", "gravar-pendrive-linux.sh",
		".DS_Store", "._README.md", "web/.DS_Store", "core/._lixo"}
	for _, p := range nao {
		if existe(filepath.Join(stage, p)) {
			t.Errorf("não deveria existir %s", p)
		}
	}
}

func TestMontarPreservaBinENoResto(t *testing.T) {
	a := novoAmbiente(t)
	stage, err := a.montar(t, false)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(stage, "antigo.txt"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(stage, "pasta-velha"), 0o755)
	os.WriteFile(filepath.Join(stage, "pasta-velha", "f"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(stage, ".oculto"), []byte("x"), 0o644)
	if _, err := a.montar(t, false); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"antigo.txt", "pasta-velha", ".oculto"} {
		if existe(filepath.Join(stage, p)) {
			t.Errorf("%s deveria ter sumido", p)
		}
	}
	if n := a.contar("/llamafile"); n != 1 {
		t.Errorf("llamafile baixado %d vezes", n)
	}
	if n := a.contar("/modelo"); n != 1 {
		t.Errorf("modelo baixado %d vezes", n)
	}
	if !existe(filepath.Join(stage, "CHECKSUMS.sha256")) {
		t.Error("CHECKSUMS.sha256 sumiu")
	}
}

// Um bin/llamafile antigo e menor na cache é retomado com Range e fica com o
// tamanho certo; só o SHA do manifesto pega o motor corrompido.
func TestMotorAntigoMenorNaCacheEhTrocado(t *testing.T) {
	a := novoAmbiente(t)
	stage, err := a.montar(t, false)
	if err != nil {
		t.Fatal(err)
	}
	llama := filepath.Join(stage, "bin", "llamafile")
	os.WriteFile(llama, []byte("motor-velho"), 0o755)
	if _, err := a.montar(t, false); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{llama, llama + ".exe"} {
		if b, _ := os.ReadFile(p); string(b) != "llamafile-conteudo" {
			t.Errorf("%s ficou com %q", filepath.Base(p), b)
		}
	}
}

func TestMontarTiraModeloAntigo(t *testing.T) {
	a := novoAmbiente(t)
	stage, err := a.montar(t, false)
	if err != nil {
		t.Fatal(err)
	}
	velho := filepath.Join(stage, "models", "velho.gguf")
	os.WriteFile(velho, []byte("x"), 0o644)
	if _, err := a.montar(t, false); err != nil {
		t.Fatal(err)
	}
	if existe(velho) {
		t.Error("models/velho.gguf deveria ter sumido")
	}
	if !existe(filepath.Join(stage, "models", "modelo.gguf")) {
		t.Error("modelo atual sumiu")
	}
}

func TestMontarModoExecutavel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sem bit de execução no Windows")
	}
	a := novoAmbiente(t)
	stage, err := a.montar(t, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"iniciar-cd2-linux.sh", "INICIAR-CD2-Mac.command"} {
		st, err := os.Stat(filepath.Join(stage, p))
		if err != nil || st.Mode().Perm()&0o100 == 0 {
			t.Errorf("%s sem bit de execução", p)
		}
	}
	st, _ := os.Stat(filepath.Join(stage, "README.md"))
	if st.Mode().Perm()&0o100 != 0 {
		t.Error("README.md não deveria ser executável")
	}
}

func TestCtlConfereComSHA256SUMS(t *testing.T) {
	a := novoAmbiente(t)
	a.arquivos["/ctl/cd2-ctl-darwin-arm64"] = []byte("adulterado")
	_, err := a.montar(t, false)
	if err == nil || !strings.Contains(err.Error(), "cd2-ctl") {
		t.Fatalf("esperava erro citando cd2-ctl, veio %v", err)
	}
	if !existe(filepath.Join(a.cache, "ctl-SHA256SUMS-0.6.0")) {
		t.Error("SHA256SUMS não ficou guardado na cache")
	}
}

func TestSemRedeComCacheCompleta(t *testing.T) {
	a := novoAmbiente(t)
	if _, err := a.montar(t, false); err != nil {
		t.Fatal(err)
	}
	a.srv.Close()
	stage, err := a.montar(t, true)
	if err != nil {
		t.Fatal(err)
	}
	if !existe(filepath.Join(stage, "models", "modelo.gguf")) {
		t.Error("modelo ausente")
	}
}

func TestSemRedeFaltando(t *testing.T) {
	a := novoAmbiente(t)
	stage, err := a.montar(t, false)
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(stage, "models", "modelo.gguf"))
	a.srv.Close()
	if _, err := a.montar(t, true); !errors.Is(err, ErrFaltaInternet) {
		t.Fatalf("err = %v", err)
	}
}

func TestSemRedeSemSHA256SUMSDoCtl(t *testing.T) {
	a := novoAmbiente(t)
	if _, err := a.montar(t, false); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(a.cache, "ctl-SHA256SUMS-0.6.0"))
	if _, err := a.montar(t, true); !errors.Is(err, ErrFaltaInternet) {
		t.Fatalf("err = %v", err)
	}
}

func TestMontarEmiteEventos(t *testing.T) {
	a := novoAmbiente(t)
	vistos := map[string]bool{}
	_, err := Montar(context.Background(), a.srv.Client(), a.m, a.cache, false, func(e Evento) { vistos[e.Item] = true })
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range []string{"zip", "motor", "modelo", "toolbox windows-amd64", "cd2-ctl darwin-arm64"} {
		if !vistos[it] {
			t.Errorf("sem evento %q", it)
		}
	}
}

func TestBytesParaBaixar(t *testing.T) {
	a := novoAmbiente(t)
	a.m.Modelo.Bytes = 22
	total := a.m.Zip.Bytes + 22 + 18
	if got := BytesParaBaixar(a.m, a.cache); got != total {
		t.Fatalf("got %d, want %d", got, total)
	}
	if _, err := a.montar(t, false); err != nil {
		t.Fatal(err)
	}
	if got := BytesParaBaixar(a.m, a.cache); got != 0 {
		t.Fatalf("depois de montar: %d", got)
	}
}

func TestChecksumsFormato(t *testing.T) {
	a := novoAmbiente(t)
	stage, err := a.montar(t, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(stage, "CHECKSUMS.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	txt := string(b)
	if !strings.HasSuffix(txt, "\n") {
		t.Error("sem \\n final")
	}
	var caminhos []string
	for _, l := range strings.Split(strings.TrimSuffix(txt, "\n"), "\n") {
		soma, cam, ok := strings.Cut(l, "  ")
		if !ok || len(soma) != 64 || soma != strings.ToLower(soma) {
			t.Fatalf("linha ruim: %q", l)
		}
		caminhos = append(caminhos, cam)
		dados, err := os.ReadFile(filepath.Join(stage, filepath.FromSlash(cam)))
		if err != nil || sha(dados) != soma {
			t.Errorf("sha de %s não confere", cam)
		}
	}
	esperado := []string{"bin/llamafile", "models/modelo.gguf"}
	for _, p := range plats {
		esperado = append(esperado, "bin/toolbox/"+p+"/toolbox"+extensao(p), "bin/ctl/"+p+"/cd2-ctl"+extensao(p))
	}
	sort.Strings(esperado)
	if strings.Join(caminhos, "|") != strings.Join(esperado, "|") {
		t.Fatalf("caminhos:\n%v\nesperado:\n%v", caminhos, esperado)
	}
}

// O Copiar pula .DS_Store e ._*; se entrassem no CHECKSUMS, toda gravação
// falharia na conferência. Como o script, só models/<modelo> entra.
func TestChecksumsSoListaOModeloAtual(t *testing.T) {
	a := novoAmbiente(t)
	stage, err := a.montar(t, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{".DS_Store", "._modelo.gguf", "LEIA.txt"} {
		os.WriteFile(filepath.Join(stage, "models", n), []byte("lixo"), 0o644)
	}
	if _, err := a.montar(t, false); err != nil {
		t.Fatal(err)
	}
	ls, err := LerChecksums(filepath.Join(stage, "CHECKSUMS.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	var modelos []string
	for _, l := range ls {
		if strings.HasPrefix(l.Caminho, "models/") {
			modelos = append(modelos, l.Caminho)
		}
	}
	if strings.Join(modelos, "|") != "models/modelo.gguf" {
		t.Fatalf("models no CHECKSUMS: %v", modelos)
	}
}

func TestLerChecksums(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.sha256")
	os.WriteFile(p, []byte("aa  bin/x\nbb *bin/y\n\n"), 0o644)
	ls, err := LerChecksums(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(ls) != 2 || ls[0] != (Linha{"aa", "bin/x"}) || ls[1] != (Linha{"bb", "bin/y"}) {
		t.Fatalf("%v", ls)
	}
}

// monta um stage com os arquivos dados (conteúdo = nome) para o arrumarLancadores.
func stageCom(t *testing.T, arquivos ...string) string {
	t.Helper()
	stage := t.TempDir()
	for _, f := range arquivos {
		p := filepath.Join(stage, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return stage
}

func TestArrumarLancadoresComLancador(t *testing.T) {
	stage := stageCom(t, "IA de Bolso.exe", "IA de Bolso.app/Contents/MacOS/IA de Bolso",
		"INICIAR-CD2-Windows.bat", "INICIAR-CD2-Mac.command", "iniciar-cd2-linux.sh", "core/start-unix.sh")
	if err := arrumarLancadores(stage); err != nil {
		t.Fatal(err)
	}
	// Até o teste no Windows, o .bat fica na raiz ao lado do .exe (alternativa).
	for _, f := range []string{"INICIAR-CD2-Windows.bat", "core/INICIAR-CD2-Mac.command", "iniciar-cd2-linux.sh", "IA de Bolso.exe"} {
		if !existe(filepath.Join(stage, filepath.FromSlash(f))) {
			t.Errorf("faltou %s", f)
		}
	}
	for _, f := range []string{"core/INICIAR-CD2-Windows.bat", "INICIAR-CD2-Mac.command"} {
		if existe(filepath.Join(stage, filepath.FromSlash(f))) {
			t.Errorf("%s não deveria existir", f)
		}
	}
}

func TestArrumarLancadoresSoWindowsBatFica(t *testing.T) {
	stage := stageCom(t, "IA de Bolso.exe", "INICIAR-CD2-Windows.bat")
	if err := arrumarLancadores(stage); err != nil {
		t.Fatal(err)
	}
	if !existe(filepath.Join(stage, "INICIAR-CD2-Windows.bat")) || existe(filepath.Join(stage, "core", "INICIAR-CD2-Windows.bat")) {
		t.Error(".bat deveria ficar na raiz")
	}
}

func TestArrumarLancadoresSemLancador(t *testing.T) {
	stage := stageCom(t, "INICIAR-CD2-Windows.bat", "INICIAR-CD2-Mac.command", "iniciar-cd2-linux.sh")
	if err := arrumarLancadores(stage); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"INICIAR-CD2-Windows.bat", "INICIAR-CD2-Mac.command", "iniciar-cd2-linux.sh"} {
		if !existe(filepath.Join(stage, f)) {
			t.Errorf("%s deveria continuar na raiz", f)
		}
	}
	if existe(filepath.Join(stage, "core")) {
		t.Error("não deveria criar core/")
	}
}

func TestArrumarLancadoresSoMac(t *testing.T) {
	stage := stageCom(t, "IA de Bolso.app/Contents/MacOS/IA de Bolso", "INICIAR-CD2-Windows.bat", "INICIAR-CD2-Mac.command")
	if err := arrumarLancadores(stage); err != nil {
		t.Fatal(err)
	}
	if !existe(filepath.Join(stage, "core", "INICIAR-CD2-Mac.command")) || existe(filepath.Join(stage, "INICIAR-CD2-Mac.command")) {
		t.Error(".command deveria estar só em core/")
	}
	if !existe(filepath.Join(stage, "INICIAR-CD2-Windows.bat")) || existe(filepath.Join(stage, "core", "INICIAR-CD2-Windows.bat")) {
		t.Error(".bat deveria ficar na raiz")
	}
}

func TestArrumarLancadoresSobrescreveEIdempotente(t *testing.T) {
	stage := stageCom(t, "IA de Bolso.app/Contents/MacOS/IA de Bolso", "INICIAR-CD2-Mac.command", "core/INICIAR-CD2-Mac.command")
	for i := 0; i < 2; i++ {
		if err := arrumarLancadores(stage); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(filepath.Join(stage, "core", "INICIAR-CD2-Mac.command"))
	if err != nil || string(b) != "INICIAR-CD2-Mac.command" {
		t.Errorf("core/ deveria ter o da raiz: %q %v", b, err)
	}
	if existe(filepath.Join(stage, "INICIAR-CD2-Mac.command")) {
		t.Error("sobrou .command na raiz")
	}
}

func TestExtrairLevaOLancador(t *testing.T) {
	a := novoAmbienteCom(t, true)
	stage, err := a.montar(t, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"IA de Bolso.exe", "IA de Bolso.app/Contents/MacOS/IA de Bolso", "IA de Bolso.app/Contents/Info.plist", "INICIAR-CD2-Windows.bat", "core/INICIAR-CD2-Mac.command"} {
		if !existe(filepath.Join(stage, filepath.FromSlash(f))) {
			t.Errorf("faltou %s", f)
		}
	}
	if existe(filepath.Join(stage, "core", "INICIAR-CD2-Windows.bat")) || existe(filepath.Join(stage, "INICIAR-CD2-Mac.command")) {
		t.Error("o .bat fica na raiz e o .command só em core/")
	}
	if existe(filepath.Join(stage, "IA de Bolso.app", "Contents", "._lixo")) {
		t.Error("lixo ._ entrou no stage")
	}
	if runtime.GOOS != "windows" {
		st, err := os.Stat(filepath.Join(stage, "IA de Bolso.app", "Contents", "MacOS", "IA de Bolso"))
		if err != nil || st.Mode().Perm() != 0o755 {
			t.Errorf("executável do .app deveria ser 0755: %v %v", st, err)
		}
	}
}

func TestChecksumsComLancador(t *testing.T) {
	a := novoAmbienteCom(t, true)
	stage, err := a.montar(t, false)
	if err != nil {
		t.Fatal(err)
	}
	ls, err := LerChecksums(filepath.Join(stage, "CHECKSUMS.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	var caminhos []string
	for _, l := range ls {
		caminhos = append(caminhos, l.Caminho)
	}
	if !sort.StringsAreSorted(caminhos) {
		t.Errorf("fora de ordem: %v", caminhos)
	}
	tem := map[string]bool{}
	for _, c := range caminhos {
		tem[c] = true
	}
	for _, c := range []string{"IA de Bolso.exe", "IA de Bolso.app/Contents/MacOS/IA de Bolso"} {
		if !tem[c] {
			t.Errorf("CHECKSUMS sem %q: %v", c, caminhos)
		}
	}
	if tem["IA de Bolso.app/Contents/Info.plist"] {
		t.Error("só o executável do .app entra")
	}
}
