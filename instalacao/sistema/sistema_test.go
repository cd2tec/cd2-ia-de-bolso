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
	rodar = func(exe string, args ...string) error {
		rodados = append(rodados, Comando{Exe: exe, Args: args})
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
	if err := s.Atalhos(`C:\Users\o'neil\AppData\Local\Programs\CD2 IA de Bolso\IA de Bolso.exe`, true); err != nil {
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
	script := c.Args[len(c.Args)-1]
	for _, trecho := range []string{
		`'C:\Users\ana\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\CD2 IA de Bolso.lnk'`,
		`TargetPath = 'C:\Users\o''neil\AppData\Local\Programs\CD2 IA de Bolso\IA de Bolso.exe'`,
		`WorkingDirectory = 'C:\Users\o''neil\AppData\Local\Programs\CD2 IA de Bolso'`,
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
	script := (*rodados)[0].Args[len((*rodados)[0].Args)-1]
	if strings.Count(script, "Remove-Item") != 2 || !strings.Contains(script, "SilentlyContinue; Remove-Item") ||
		strings.Contains(script, ".Save()") {
		t.Errorf("script de remoção:\n%s", script)
	}
}

func TestPlanoDeRemocaoWindowsAgendaDepoisDeSair(t *testing.T) {
	trocar(t, `C:\Users\ana`, envWin)
	destino := `C:\Users\ana\AppData\Local\Programs\CD2 IA de Bolso`
	c := Do("windows").PlanoRemocao(destino, 4242)
	if c.Exe != "cmd.exe" || len(c.Args) != 2 || c.Args[0] != "/c" {
		t.Fatalf("comando = %#v", c)
	}
	s := c.Args[1]
	espera := strings.Index(s, `tasklist /FI "PID eq 4242"`)
	apaga := strings.Index(s, `rmdir /s /q "`+destino+`"`)
	if espera < 0 || apaga < 0 || espera > apaga {
		t.Fatalf("quero o laço do tasklist antes do rmdir:\n%s", s)
	}
	for _, trecho := range []string{"(1,1,30)", "timeout /t 1 /nobreak >nul", `find " 4242 "`} {
		if !strings.Contains(s, trecho) {
			t.Errorf("faltou %q em:\n%s", trecho, s)
		}
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

func TestAtalhoMacPeloFinder(t *testing.T) {
	home := t.TempDir()
	_, rodados := trocar(t, home, func(string) string { return "" })
	s := Do("darwin")
	lanc := `/Users/ana/Applications/IA de Bolso/IA de Bolso.app/Contents/MacOS/IA de Bolso`
	if err := s.Atalhos(lanc, true); err != nil {
		t.Fatal(err)
	}
	if len(*rodados) != 1 || (*rodados)[0].Exe != "/usr/bin/osascript" {
		t.Fatalf("comandos = %#v", *rodados)
	}
	script := strings.Join((*rodados)[0].Args, " ")
	for _, trecho := range []string{
		`tell application "Finder" to make alias file to POSIX file "/Users/ana/Applications/IA de Bolso/IA de Bolso.app"`,
		`at POSIX file "` + home + `/Desktop"`,
		`{name:"IA de Bolso"}`,
	} {
		if !strings.Contains(script, trecho) {
			t.Errorf("faltou %q em %s", trecho, script)
		}
	}

	// Sem área de trabalho: não chama o Finder e tira o apelido que houver.
	apelido := filepath.Join(home, "Desktop", "IA de Bolso")
	if err := os.MkdirAll(filepath.Dir(apelido), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(apelido, []byte("alias"), 0o644); err != nil {
		t.Fatal(err)
	}
	*rodados = nil
	if err := s.Atalhos(lanc, false); err != nil {
		t.Fatal(err)
	}
	if len(*rodados) != 0 {
		t.Errorf("chamou o Finder sem área de trabalho: %#v", *rodados)
	}
	if _, err := os.Stat(apelido); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("apelido ficou: %v", err)
	}

	// Remover também tira o apelido, e não reclama se ele já não existe.
	if err := os.WriteFile(apelido, []byte("alias"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Remover(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(apelido); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Remover deixou o apelido: %v", err)
	}
	if err := s.Remover(); err != nil {
		t.Fatal(err)
	}
}

func TestPlanoDeRemocaoMac(t *testing.T) {
	c := Do("darwin").PlanoRemocao("/Users/d'ana/Applications/IA de Bolso", 77)
	if c.Exe != "/bin/sh" || len(c.Args) != 2 || c.Args[0] != "-c" {
		t.Fatalf("comando = %#v", c)
	}
	s := c.Args[1]
	espera := strings.Index(s, "kill -0 77")
	apaga := strings.Index(s, `rm -rf '/Users/d'\''ana/Applications/IA de Bolso'`)
	if espera < 0 || apaga < 0 || espera > apaga {
		t.Errorf("quero esperar o 77 antes do rm -rf:\n%s", s)
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
	if err := rodar("/bin/sleep", "5"); err == nil {
		t.Fatal("comando travado não deu erro")
	}
	if time.Since(inicio) > 3*time.Second {
		t.Errorf("demorou %v para cortar", time.Since(inicio))
	}
}
