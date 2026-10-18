package sistema

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"howett.net/plist"
)

func TestDestinos(t *testing.T) {
	env := func(k string) string {
		if k == "LOCALAPPDATA" {
			return `C:\Users\ana\AppData\Local`
		}
		return ""
	}
	casos := []struct{ goos, destino, lancador string }{
		{"windows", `C:\Users\ana\AppData\Local\Programs\CD2 IA de Bolso`,
			`C:\Users\ana\AppData\Local\Programs\CD2 IA de Bolso\IA de Bolso.exe`},
		{"darwin", "/Users/ana/Applications/IA de Bolso",
			"/Users/ana/Applications/IA de Bolso/IA de Bolso.app/Contents/MacOS/IA de Bolso"},
	}
	for _, c := range casos {
		d := Destino(c.goos, env, "/Users/ana")
		if d != c.destino {
			t.Errorf("Destino(%s) = %q, quero %q", c.goos, d, c.destino)
		}
		if l := Lancador(d, c.goos); l != c.lancador {
			t.Errorf("Lancador(%s) = %q, quero %q", c.goos, l, c.lancador)
		}
	}
	semEnv := func(string) string { return "" }
	if d := Destino("windows", semEnv, `C:\Users\ana`); d != `C:\Users\ana\AppData\Local\Programs\CD2 IA de Bolso` {
		t.Errorf("Destino sem LOCALAPPDATA = %q", d)
	}
}

// regFalso guarda os valores gravados no registro, por chave.
type regFalso struct {
	valores map[string]map[string]any
}

func novoReg() *regFalso { return &regFalso{valores: map[string]map[string]any{}} }

func (r *regFalso) GravarTexto(chave, nome, valor string) error {
	if r.valores[chave] == nil {
		r.valores[chave] = map[string]any{}
	}
	r.valores[chave][nome] = valor
	return nil
}

func (r *regFalso) GravarNumero(chave, nome string, valor uint32) error {
	if r.valores[chave] == nil {
		r.valores[chave] = map[string]any{}
	}
	r.valores[chave][nome] = valor
	return nil
}

func (r *regFalso) Existe(chave, nome string) bool {
	_, ok := r.valores[chave][nome]
	return ok
}

func (r *regFalso) ApagarValor(chave, nome string) error {
	delete(r.valores[chave], nome)
	return nil
}

func (r *regFalso) ApagarChave(chave string) error {
	delete(r.valores, chave)
	return nil
}

// trocar redireciona os efeitos de sistema para o teste e devolve os
// comandos que seriam rodados.
func trocar(t *testing.T, home string, env func(string) string) (*regFalso, *[]Comando) {
	t.Helper()
	reg := novoReg()
	var rodados []Comando
	velhoReg, velhoRodar, velhoHome, velhoEnv := registro, rodar, pastaPessoal, ambiente
	registro = reg
	rodar = func(c Comando) error {
		rodados = append(rodados, c)
		return nil
	}
	pastaPessoal = func() string { return home }
	ambiente = env
	t.Cleanup(func() { registro, rodar, pastaPessoal, ambiente = velhoReg, velhoRodar, velhoHome, velhoEnv })
	return reg, &rodados
}

const lancWin = `C:\Users\ana\AppData\Local\Programs\CD2 IA de Bolso\IA de Bolso.exe`

func envWin(k string) string {
	switch k {
	case "APPDATA":
		return `C:\Users\ana\AppData\Roaming`
	case "LOCALAPPDATA":
		return `C:\Users\ana\AppData\Local`
	}
	return ""
}

func TestInicioWindowsUsaRunDoUsuario(t *testing.T) {
	reg, _ := trocar(t, `C:\Users\ana`, envWin)
	s := Do("windows")
	if s.InicioLigado() {
		t.Fatal("início ligado antes de ligar")
	}
	if err := s.LigarInicio(lancWin, true); err != nil {
		t.Fatal(err)
	}
	chave := `Software\Microsoft\Windows\CurrentVersion\Run`
	v, ok := reg.valores[chave]["CD2 IA de Bolso"]
	if !ok {
		t.Fatalf("faltou o valor em HKCU\\%s: %#v", chave, reg.valores)
	}
	if quer := `"` + lancWin + `" --inicio`; v != quer {
		t.Errorf("valor = %q, quero %q", v, quer)
	}
	if !s.InicioLigado() {
		t.Error("InicioLigado = false depois de ligar")
	}
	if err := s.LigarInicio(lancWin, false); err != nil {
		t.Fatal(err)
	}
	if s.InicioLigado() {
		t.Error("InicioLigado = true depois de desligar")
	}
	// Desligar de novo não é erro.
	if err := s.LigarInicio(lancWin, false); err != nil {
		t.Fatal(err)
	}
}

func TestRegistroWindowsParaDesinstalar(t *testing.T) {
	reg, _ := trocar(t, `C:\Users\ana`, envWin)
	s := Do("windows")
	if err := s.Registrar(lancWin, "1.4.0"); err != nil {
		t.Fatal(err)
	}
	chave := `Software\Microsoft\Windows\CurrentVersion\Uninstall\CD2IADeBolso`
	quer := map[string]any{
		"DisplayName":     "CD2 IA de Bolso",
		"DisplayVersion":  "1.4.0",
		"Publisher":       "CD2 Tech",
		"UninstallString": `"` + lancWin + `" --desinstalar`,
		"DisplayIcon":     `"` + lancWin + `"`,
		"InstallLocation": `C:\Users\ana\AppData\Local\Programs\CD2 IA de Bolso`,
		"NoModify":        uint32(1),
	}
	for nome, v := range quer {
		if reg.valores[chave][nome] != v {
			t.Errorf("%s = %#v, quero %#v", nome, reg.valores[chave][nome], v)
		}
	}
	if err := s.Remover(); err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.valores[chave]; ok {
		t.Error("Remover deixou a chave de desinstalar")
	}
}

func TestAtalhosWindowsPorPowerShell(t *testing.T) {
	_, rodados := trocar(t, `C:\Users\ana`, envWin)
	s := Do("windows")
	lanc := `C:\Users\o'neil’s\AppData\Local\Programs\CD2 IA de Bolso\IA de Bolso.exe`
	if err := s.Atalhos(lanc, true); err != nil {
		t.Fatal(err)
	}
	if len(*rodados) != 1 {
		t.Fatalf("rodou %d comandos, quero 1", len(*rodados))
	}
	c := (*rodados)[0]
	if c.Exe != "powershell.exe" {
		t.Errorf("Exe = %q", c.Exe)
	}
	if len(c.Args) < 2 || c.Args[len(c.Args)-2] != "-Command" {
		t.Fatalf("args = %q", c.Args)
	}
	for _, a := range []string{"-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden"} {
		if !contem(c.Args, a) {
			t.Errorf("faltou %s em %q", a, c.Args)
		}
	}
	for _, e := range []string{
		`CD2_MENU=C:\Users\ana\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\CD2 IA de Bolso.lnk`,
		"CD2_LANCADOR=" + lanc,
		`CD2_PASTA=C:\Users\o'neil’s\AppData\Local\Programs\CD2 IA de Bolso`,
	} {
		if !contem(c.Env, e) {
			t.Errorf("faltou %q no ambiente %q", e, c.Env)
		}
	}
	script := c.Args[len(c.Args)-1]
	if strings.Contains(script, `C:\`) || strings.Contains(script, "neil") {
		t.Errorf("caminho dentro do script:\n%s", script)
	}
	for _, trecho := range []string{
		"CreateShortcut($env:CD2_MENU)",
		"TargetPath = $env:CD2_LANCADOR",
		"WorkingDirectory = $env:CD2_PASTA",
		`GetFolderPath('Desktop')) 'CD2 IA de Bolso.lnk'`,
		"WScript.Shell",
	} {
		if !strings.Contains(script, trecho) {
			t.Errorf("script sem %q:\n%s", trecho, script)
		}
	}
	if strings.Count(script, ".Save()") != 2 {
		t.Errorf("quero dois atalhos salvos:\n%s", script)
	}

	// Sem área de trabalho: só o Menu Iniciar, e o da área de trabalho sai.
	*rodados = nil
	if err := s.Atalhos(lancWin, false); err != nil {
		t.Fatal(err)
	}
	script = (*rodados)[0].Args[len((*rodados)[0].Args)-1]
	if strings.Count(script, ".Save()") != 1 || !strings.Contains(script, "Remove-Item") {
		t.Errorf("sem área de trabalho, quero um atalho e a remoção do outro:\n%s", script)
	}
}

func TestRemoverWindowsTiraOsAtalhos(t *testing.T) {
	_, rodados := trocar(t, `C:\Users\ana`, envWin)
	if err := Do("windows").Remover(); err != nil {
		t.Fatal(err)
	}
	if len(*rodados) != 1 {
		t.Fatalf("rodou %d comandos, quero 1", len(*rodados))
	}
	c := (*rodados)[0]
	script := c.Args[len(c.Args)-1]
	if strings.Count(script, "Remove-Item") != 2 || !strings.Contains(script, "SilentlyContinue; Remove-Item") ||
		strings.Contains(script, ".Save()") || !strings.Contains(script, "$env:CD2_MENU") {
		t.Errorf("script de remoção:\n%s", script)
	}
	if !contem(c.Env, `CD2_MENU=C:\Users\ana\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\CD2 IA de Bolso.lnk`) {
		t.Errorf("ambiente = %q", c.Env)
	}
}

func TestPlanoDeRemocaoWindowsAgendaDepoisDeSair(t *testing.T) {
	trocar(t, `C:\Users\ana`, envWin)
	destino := `C:\Users\ana\AppData\Local\Programs\CD2 IA de Bolso`
	c := Do("windows").PlanoRemocao(destino, 4242)
	if c.Exe != "powershell.exe" || !contem(c.Args, "-NonInteractive") || !contem(c.Args, "Hidden") {
		t.Fatalf("comando = %#v", c)
	}
	s := c.Args[len(c.Args)-1]
	espera := strings.Index(s, "Wait-Process -Id 4242 -Timeout 30")
	apaga := strings.Index(s, "Remove-Item -LiteralPath $env:CD2_DESTINO -Recurse -Force")
	if espera < 0 || apaga < 0 || espera > apaga {
		t.Fatalf("quero o Wait-Process antes do Remove-Item:\n%s", s)
	}
	if strings.Contains(s, destino) {
		t.Errorf("caminho dentro do script:\n%s", s)
	}
	if !contem(c.Env, "CD2_DESTINO="+destino) {
		t.Errorf("ambiente = %q", c.Env)
	}
}

func TestPlanoDeRemocaoRecusaPastaEstranha(t *testing.T) {
	for _, c := range []struct {
		goos, destino string
	}{
		{"windows", `C:\Users\ana`},
		{"windows", `Programs\CD2 IA de Bolso`},
		{"windows", ``},
		{"darwin", "/Users/ana"},
		{"darwin", "Applications/IA de Bolso"},
		{"darwin", ""},
	} {
		if got := Do(c.goos).PlanoRemocao(c.destino, 10); got.Exe != "" {
			t.Errorf("%s %q: devia recusar, veio %#v", c.goos, c.destino, got)
		}
	}
	if got := Do("darwin").PlanoRemocao("/Users/ana/Applications/IA de Bolso", 0); got.Exe != "" {
		t.Errorf("pid 0 devia recusar: %#v", got)
	}
}

func TestLaunchAgentMac(t *testing.T) {
	home := t.TempDir()
	trocar(t, home, func(string) string { return "" })
	s := Do("darwin")
	lanc := Lancador(Destino("darwin", nil, home), "darwin")
	if s.InicioLigado() {
		t.Fatal("início ligado antes de ligar")
	}
	if err := s.LigarInicio(lanc, true); err != nil {
		t.Fatal(err)
	}
	arq := filepath.Join(home, "Library", "LaunchAgents", "br.com.cd2.iadebolso.plist")
	b, err := os.ReadFile(arq)
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Label            string
		ProgramArguments []string
		RunAtLoad        bool
	}
	if _, err := plist.Unmarshal(b, &p); err != nil {
		t.Fatalf("plist não lê de volta: %v\n%s", err, b)
	}
	if p.Label != "br.com.cd2.iadebolso" || !p.RunAtLoad ||
		len(p.ProgramArguments) != 2 || p.ProgramArguments[0] != lanc || p.ProgramArguments[1] != "--inicio" {
		t.Errorf("plist = %#v", p)
	}
	if !bytes.Contains(b, []byte("<true/>")) {
		t.Errorf("RunAtLoad não está true:\n%s", b)
	}
	if !s.InicioLigado() {
		t.Error("InicioLigado = false depois de ligar")
	}
	if err := s.LigarInicio(lanc, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(arq); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("plist ficou: %v", err)
	}
	if err := s.LigarInicio(lanc, false); err != nil {
		t.Errorf("desligar de novo deu erro: %v", err)
	}
}

func TestAtalhoMacComLinkSimbolico(t *testing.T) {
	home := t.TempDir()
	_, rodados := trocar(t, home, func(string) string { return "" })
	if err := os.MkdirAll(filepath.Join(home, "Desktop"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := Do("darwin")
	lanc := `/Users/ana/Applications/IA de Bolso/IA de Bolso.app/Contents/MacOS/IA de Bolso`
	apelido := filepath.Join(home, "Desktop", "IA de Bolso")
	for i := 0; i < 2; i++ { // a segunda vez troca o link que já existe
		if err := s.Atalhos(lanc, true); err != nil {
			t.Fatal(err)
		}
	}
	if len(*rodados) != 0 {
		t.Errorf("rodou comandos: %#v", *rodados)
	}
	alvo, err := os.Readlink(apelido)
	if err != nil || alvo != "/Users/ana/Applications/IA de Bolso/IA de Bolso.app" {
		t.Fatalf("link = %q, %v", alvo, err)
	}

	// Sem área de trabalho: tira o link.
	if err := s.Atalhos(lanc, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(apelido); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("link ficou: %v", err)
	}

	// Remover também tira o link, e não reclama se ele já não existe.
	if err := s.Atalhos(lanc, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Remover(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(apelido); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Remover deixou o link: %v", err)
	}
	if err := s.Remover(); err != nil {
		t.Fatal(err)
	}
}

func TestAtalhoMacNaoApagaArquivoDaPessoa(t *testing.T) {
	home := t.TempDir()
	trocar(t, home, func(string) string { return "" })
	arq := filepath.Join(home, "Desktop", "IA de Bolso")
	if err := os.MkdirAll(filepath.Dir(arq), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(arq, []byte("meu"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Do("darwin")
	if err := s.Remover(); err != nil {
		t.Fatal(err)
	}
	if err := s.Atalhos("/x/IA de Bolso.app/Contents/MacOS/IA de Bolso", false); err != nil {
		t.Fatal(err)
	}
	if err := s.Atalhos("/x/IA de Bolso.app/Contents/MacOS/IA de Bolso", true); err == nil {
		t.Error("devia avisar que não criou o link por cima do arquivo")
	}
	if b, err := os.ReadFile(arq); err != nil || string(b) != "meu" {
		t.Errorf("o arquivo da pessoa mudou: %q, %v", b, err)
	}
}

func TestPlanoDeRemocaoMac(t *testing.T) {
	destino := "/Users/d'ana/Applications/IA de Bolso"
	c := Do("darwin").PlanoRemocao(destino, 77)
	if c.Exe != "/bin/sh" || len(c.Args) != 2 || c.Args[0] != "-c" {
		t.Fatalf("comando = %#v", c)
	}
	s := c.Args[1]
	espera := strings.Index(s, "kill -0 77")
	apaga := strings.Index(s, `rm -rf "$CD2_DESTINO"`)
	if espera < 0 || apaga < 0 || espera > apaga {
		t.Errorf("quero esperar o 77 antes do rm -rf:\n%s", s)
	}
	if strings.Contains(s, "ana") || !contem(c.Env, "CD2_DESTINO="+destino) {
		t.Errorf("caminho fora do ambiente: %#v", c)
	}
}

func TestPlanoDeRemocaoMacDeVerdade(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("sem /bin/sh")
	}
	destino := filepath.Join(t.TempDir(), "d'a \"$x", "IA de Bolso")
	if err := os.MkdirAll(filepath.Join(destino, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	irma := filepath.Join(filepath.Dir(destino), "fica.txt")
	if err := os.WriteFile(irma, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Do("darwin").PlanoRemocao(destino, 999999)
	if err := rodar(c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(destino); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("pasta ficou: %v", err)
	}
	if _, err := os.Stat(irma); err != nil {
		t.Errorf("apagou o que não devia: %v", err)
	}
}

func TestOutroSistemaRecusa(t *testing.T) {
	s := Do("linux")
	if err := s.Atalhos("/x", true); err == nil {
		t.Error("Atalhos no linux devia dar erro")
	}
	if s.InicioLigado() {
		t.Error("InicioLigado no linux")
	}
}

func contem(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func TestRodarCortaComandoTravado(t *testing.T) {
	if _, err := os.Stat("/bin/sleep"); err != nil {
		t.Skip("sem /bin/sleep")
	}
	velho := limiteComando
	limiteComando = 100 * time.Millisecond
	t.Cleanup(func() { limiteComando = velho })
	inicio := time.Now()
	if err := rodar(Comando{Exe: "/bin/sleep", Args: []string{"5"}}); err == nil {
		t.Fatal("comando travado não deu erro")
	}
	if time.Since(inicio) > 3*time.Second {
		t.Errorf("demorou %v para cortar", time.Since(inicio))
	}
}
