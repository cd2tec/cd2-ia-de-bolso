package toolbox

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/dbconf"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/faketoolbox"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/secret"
	"github.com/xuri/excelize/v2"
)

// Com CD2_FAKE_TOOLBOX=1 este binário de teste é o toolbox.
func TestMain(m *testing.M) {
	if os.Getenv(faketoolbox.EnvVar) == "1" {
		faketoolbox.Main()
		return
	}
	os.Exit(m.Run())
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func portFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

func newSup(t *testing.T, ready time.Duration) *Supervisor {
	t.Helper()
	t.Setenv(faketoolbox.EnvVar, "1")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{
		Source: self, CacheDir: t.TempDir(), RecipeDir: filepath.Join("raiz", "core", "toolbox"),
		Port: freePort(t), HarnessPort: 18080, ReadyTimeout: ready, StopTimeout: 5 * time.Second,
	})
	t.Cleanup(func() { _ = s.Stop() })
	return s
}

func cfg(pw string) dbconf.Config {
	return dbconf.Config{Engine: "postgres", Host: "h", Port: "5432", Name: "d", User: "u", Password: pw, Schema: "public"}
}

func startErr(t *testing.T, err error) *StartError {
	t.Helper()
	var se *StartError
	if !errors.As(err, &se) {
		t.Fatalf("erro %v, quero *StartError", err)
	}
	return se
}

func TestSobeConfereDerrubaESobeDeNovo(t *testing.T) {
	s := newSup(t, 10*time.Second)
	if err := s.Start(cfg("boa")); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !s.Running() {
		t.Fatal("deveria estar rodando")
	}
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", s.opt.Port))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET / = %d", resp.StatusCode)
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if s.Running() || !portFree(s.opt.Port) {
		t.Error("depois do Stop: ainda rodando ou porta presa")
	}
	if err := s.Start(cfg("boa2")); err != nil {
		t.Fatalf("segundo Start na mesma porta: %v", err)
	}
}

func TestPassaFlagsEAmbienteSemAChave(t *testing.T) {
	s := newSup(t, 10*time.Second)
	rep := filepath.Join(t.TempDir(), "rep.json")
	t.Setenv(faketoolbox.ReportVar, rep)
	t.Setenv("CD2_CTL_KEY", "nao-pode-vazar")
	t.Setenv("CD2_DB_HOST", "lixo-do-ambiente")
	if err := s.Start(cfg(`a\b"c`)); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(rep)
	if err != nil {
		t.Fatal(err)
	}
	var r faketoolbox.Report
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	p := strconv.Itoa(s.opt.Port)
	want := []string{
		"--config", filepath.Join("raiz", "core", "toolbox", "postgres.yaml"),
		"--address", "127.0.0.1", "--port", p,
		"--allowed-hosts", "127.0.0.1:" + p + ",localhost:" + p,
		"--allowed-origins", "http://127.0.0.1:18080,http://localhost:18080",
		"--disable-reload",
	}
	if !reflect.DeepEqual(r.Args, want) {
		t.Errorf("args:\n got %q\nwant %q", r.Args, want)
	}
	if r.Password != `a\\b\"c` {
		t.Errorf("senha chegou %q, quero escapada para YAML", r.Password)
	}
	if r.HasKey {
		t.Error("CD2_CTL_KEY chegou ao toolbox")
	}
	if r.Engine != "postgres" || r.Host != "h" {
		t.Errorf("ambiente: engine %q host %q", r.Engine, r.Host)
	}
	if _, err := os.Stat(filepath.Join(s.opt.CacheDir, exeName())); err != nil {
		t.Errorf("o toolbox não rodou da cópia no cache: %v", err)
	}
}

func TestToolboxQueMorreNaoVazaASenha(t *testing.T) {
	s := newSup(t, 10*time.Second)
	pw := `morre"x\y`
	se := startErr(t, s.Start(cfg(pw)))
	if se.Msg != MsgDied {
		t.Errorf("Msg = %q", se.Msg)
	}
	logb, err := os.ReadFile(s.LogPath())
	if err != nil {
		t.Fatal(err)
	}
	for name, txt := range map[string]string{"Log": strings.Join(se.Log, "\n"), "toolbox.log": string(logb)} {
		for _, bad := range []string{pw, secret.YamlDQ(pw)} {
			if strings.Contains(txt, bad) {
				t.Errorf("%s contém a senha %q", name, bad)
			}
		}
		if strings.Contains(strings.ToLower(txt), "password") {
			t.Errorf("%s tem linha com password", name)
		}
		if !strings.Contains(txt, "Error: toolbox failed to initialize") {
			t.Errorf("%s perdeu a linha útil: %q", name, txt)
		}
	}
	if last := se.Log[len(se.Log)-1]; last != "(3 linha(s) omitida(s) por citar a senha)" {
		t.Errorf("última linha do Log = %q", last)
	}
	if s.Running() {
		t.Error("não deveria estar rodando")
	}
}

func TestPrazoEstourado(t *testing.T) {
	s := newSup(t, 1*time.Second)
	se := startErr(t, s.Start(cfg("lenta")))
	if !strings.Contains(se.Msg, "não respondeu em 1 segundos") {
		t.Errorf("Msg = %q", se.Msg)
	}
	if s.Running() || !portFree(s.opt.Port) {
		t.Error("o processo lento deveria ter sido derrubado")
	}
}

func TestConexaoComIsError(t *testing.T) {
	s := newSup(t, 10*time.Second)
	se := startErr(t, s.Start(cfg("iserror")))
	if se.Msg != MsgToolError {
		t.Errorf("Msg = %q", se.Msg)
	}
	if s.Running() {
		t.Error("não deveria estar rodando")
	}
}

func TestBinarioAusente(t *testing.T) {
	s := newSup(t, 10*time.Second)
	s.opt.Source = filepath.Join(t.TempDir(), "nada")
	se := startErr(t, s.Start(cfg("boa")))
	if !strings.Contains(se.Msg, "não encontrado") {
		t.Errorf("Msg = %q", se.Msg)
	}
}

func TestStartDuasVezes(t *testing.T) {
	s := newSup(t, 10*time.Second)
	if err := s.Start(cfg("boa")); err != nil {
		t.Fatal(err)
	}
	startErr(t, s.Start(cfg("boa")))
	if !s.Running() {
		t.Error("o primeiro deveria seguir rodando")
	}
}

// db_connection_status levando 3 s (mais que o limite do GET /) tem de passar dentro do prazo.
func TestConexaoLentaPassaDentroDoPrazo(t *testing.T) {
	s := newSup(t, 15*time.Second)
	t0 := time.Now()
	if err := s.Start(cfg("demora")); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if d := time.Since(t0); d < 3*time.Second || d > 15*time.Second {
		t.Errorf("levou %v, esperava entre 3 s e o prazo de 15 s", d)
	}
	if !s.Running() {
		t.Error("deveria estar rodando")
	}
}

// Uma chamada MCP pendurada não pode estourar o prazo total.
func TestPrazoRespeitadoComChamadaPendurada(t *testing.T) {
	s := newSup(t, 2*time.Second)
	t0 := time.Now()
	se := startErr(t, s.Start(cfg("demora")))
	waited := time.Since(t0)
	if !strings.Contains(se.Msg, "não respondeu em 2 segundos") {
		t.Errorf("Msg = %q", se.Msg)
	}
	// Start inclui o Stop (até 3 s de TERM); o que importa é a espera pela prontidão
	if waited > 2*time.Second+1500*time.Millisecond {
		t.Errorf("Start levou %v", waited)
	}
}

// cfgExcel é a configuração do motor excel: só a base, que é a pasta das planilhas.
func cfgExcel(pasta string) dbconf.Config {
	return dbconf.Config{Engine: "excel", Name: pasta}
}

// novaPlanilha grava um .xlsx em caminho; preencher mexe no arquivo antes de salvar (aba inicial "Sheet1").
func novaPlanilha(t *testing.T, caminho string, preencher func(f *excelize.File)) {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	preencher(f)
	if err := f.SaveAs(caminho); err != nil {
		t.Fatal(err)
	}
}

func deve(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestStartExcelImportaEmDuasEtapasEGuardaRelatorio(t *testing.T) {
	s := newSup(t, 10*time.Second)
	rep := filepath.Join(t.TempDir(), "rep.json")
	t.Setenv(faketoolbox.ReportVar, rep)
	pasta := t.TempDir()
	novaPlanilha(t, filepath.Join(pasta, "vendas.xlsx"), func(f *excelize.File) {
		deve(t, f.SetSheetRow("Sheet1", "A1", &[]any{"Produto", "Qtd"}))
		deve(t, f.SetSheetRow("Sheet1", "A2", &[]any{"Arroz", 3}))
		_, err := f.NewSheet("Rascunho")
		deve(t, err)
		deve(t, f.SetSheetRow("Rascunho", "A1", &[]any{"Ideia", "Prazo"}))
		deve(t, f.SetSheetVisible("Rascunho", false))
	})
	if s.Planilhas() != nil {
		t.Fatal("antes de qualquer Start não deveria haver relatório")
	}
	if err := s.Start(cfgExcel(pasta)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	destino := filepath.Join(s.opt.CacheDir, "excel.sqlite")
	if _, err := os.Stat(destino); err != nil {
		t.Fatalf("excel.sqlite não existe: %v", err)
	}
	for _, sobra := range []string{".novo", ".novo-wal", ".novo-shm"} {
		if _, err := os.Stat(destino + sobra); !os.IsNotExist(err) {
			t.Errorf("sobrou excel.sqlite%s (err %v)", sobra, err)
		}
	}
	rel := s.Planilhas()
	if rel == nil || rel.Tabelas != 1 || len(rel.Ignoradas) != 1 {
		t.Fatalf("relatório: %+v", rel)
	}
	b, err := os.ReadFile(rep)
	if err != nil {
		t.Fatal(err)
	}
	var r faketoolbox.Report
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	if r.Sqlite != destino {
		t.Errorf("CD2_DB_SQLITE chegou %q, quero %q", r.Sqlite, destino)
	}
	if r.Engine != "excel" {
		t.Errorf("CD2_DB_ENGINE chegou %q", r.Engine)
	}
	if want := filepath.Join("raiz", "core", "toolbox", "excel.yaml"); len(r.Args) < 2 || r.Args[1] != want {
		t.Errorf("receita: %q, quero %q", r.Args, want)
	}
}

func TestStartExcelFalhaNaImportacaoMantemCopiaAnterior(t *testing.T) {
	s := newSup(t, 10*time.Second)
	destino := filepath.Join(s.opt.CacheDir, "excel.sqlite")
	db, err := sql.Open("sqlite", destino)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE antiga (x TEXT); INSERT INTO antiga VALUES ('conteudo anterior')`)
	deve(t, err)
	deve(t, db.Close())

	se := startErr(t, s.Start(cfgExcel(filepath.Join(t.TempDir(), "nao-existe"))))
	if se.Msg != "Não achei a pasta das planilhas." {
		t.Errorf("mensagem: %q", se.Msg)
	}
	if s.Planilhas() != nil {
		t.Errorf("pasta inexistente não tem relatório: %+v", s.Planilhas())
	}
	db, err = sql.Open("sqlite", destino)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var x string
	if err := db.QueryRow(`SELECT x FROM antiga`).Scan(&x); err != nil || x != "conteudo anterior" {
		t.Errorf("a cópia anterior mudou: %q (%v)", x, err)
	}
	if _, err := os.Stat(destino + ".novo"); !os.IsNotExist(err) {
		t.Errorf("sobrou excel.sqlite.novo (err %v)", err)
	}
}

func TestStartExcelTudoIgnoradoGuardaRelatorio(t *testing.T) {
	s := newSup(t, 10*time.Second)
	pasta := t.TempDir()
	novaPlanilha(t, filepath.Join(pasta, "lembretes.xlsx"), func(f *excelize.File) {
		deve(t, f.SetCellValue("Sheet1", "A1", "anotações"))
		deve(t, f.SetCellValue("Sheet1", "A5", "ligar para o João"))
	})
	startErr(t, s.Start(cfgExcel(pasta)))
	rel := s.Planilhas()
	if rel == nil || len(rel.Ignoradas) != 1 || rel.Tabelas != 0 {
		t.Fatalf("relatório: %+v", rel)
	}
	if s.Running() {
		t.Error("não deveria ter subido o toolbox")
	}
	if _, err := os.Stat(filepath.Join(s.opt.CacheDir, "excel.sqlite.novo")); !os.IsNotExist(err) {
		t.Errorf("sobrou excel.sqlite.novo (err %v)", err)
	}
}

func TestPlanilhasZeraNosOutrosMotores(t *testing.T) {
	s := newSup(t, 10*time.Second)
	pasta := t.TempDir()
	novaPlanilha(t, filepath.Join(pasta, "a.xlsx"), func(f *excelize.File) {
		deve(t, f.SetSheetRow("Sheet1", "A1", &[]any{"Produto", "Qtd"}))
		deve(t, f.SetSheetRow("Sheet1", "A2", &[]any{"Arroz", 3}))
	})
	if err := s.Start(cfgExcel(pasta)); err != nil {
		t.Fatal(err)
	}
	if s.Planilhas() == nil {
		t.Fatal("deveria ter relatório")
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(cfg("boa")); err != nil {
		t.Fatal(err)
	}
	if s.Planilhas() != nil {
		t.Error("com outro motor o relatório deveria ser nulo")
	}
}

// planilhaSimples grava um .xlsx com uma aba de cabeçalho e uma linha (vira uma tabela).
func planilhaSimples(t *testing.T, caminho string) {
	t.Helper()
	novaPlanilha(t, caminho, func(f *excelize.File) {
		deve(t, f.SetSheetRow("Sheet1", "A1", &[]any{"Produto", "Qtd"}))
		deve(t, f.SetSheetRow("Sheet1", "A2", &[]any{"Arroz", 3}))
	})
}

// contaTabelas abre o SQLite em caminho e conta as tabelas.
func contaTabelas(t *testing.T, caminho string) int {
	t.Helper()
	db, err := sql.Open("sqlite", caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestReloadTrocaACopiaComToolboxDePe(t *testing.T) {
	s := newSup(t, 10*time.Second)
	pasta := t.TempDir()
	planilhaSimples(t, filepath.Join(pasta, "vendas.xlsx"))
	if err := s.Start(cfgExcel(pasta)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	destino := filepath.Join(s.opt.CacheDir, "excel.sqlite")
	if n := contaTabelas(t, destino); n != 1 {
		t.Fatalf("antes do Reload: %d tabelas", n)
	}
	planilhaSimples(t, filepath.Join(pasta, "estoque.xlsx"))
	if err := s.Reload(cfgExcel(pasta)); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if !s.Running() {
		t.Fatal("depois do Reload o toolbox deveria estar de pé")
	}
	if n := contaTabelas(t, destino); n != 2 {
		t.Errorf("depois do Reload: %d tabelas, quero 2", n)
	}
	if rel := s.Planilhas(); rel == nil || rel.Tabelas != 2 {
		t.Errorf("relatório: %+v", rel)
	}
	if _, err := os.Stat(destino + ".novo"); !os.IsNotExist(err) {
		t.Errorf("sobrou excel.sqlite.novo (err %v)", err)
	}
}

func TestReloadQueFalhaNaImportacaoNaoDerruba(t *testing.T) {
	s := newSup(t, 10*time.Second)
	pasta := filepath.Join(t.TempDir(), "planilhas")
	deve(t, os.Mkdir(pasta, 0o755))
	planilhaSimples(t, filepath.Join(pasta, "vendas.xlsx"))
	if err := s.Start(cfgExcel(pasta)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	deve(t, os.RemoveAll(pasta))
	se := startErr(t, s.Reload(cfgExcel(pasta)))
	if se.Msg != "Não achei a pasta das planilhas." {
		t.Errorf("mensagem: %q", se.Msg)
	}
	if !s.Running() {
		t.Error("a importação falhou e o toolbox caiu")
	}
	destino := filepath.Join(s.opt.CacheDir, "excel.sqlite")
	if n := contaTabelas(t, destino); n != 1 {
		t.Errorf("a cópia no ar mudou: %d tabelas, quero 1", n)
	}
	if _, err := os.Stat(destino + ".novo"); !os.IsNotExist(err) {
		t.Errorf("sobrou excel.sqlite.novo (err %v)", err)
	}
}

func TestReloadDeMotorQueNaoEDeArquivo(t *testing.T) {
	s := newSup(t, 10*time.Second)
	if err := s.Start(cfg("boa")); err != nil {
		t.Fatal(err)
	}
	se := startErr(t, s.Reload(cfg("boa")))
	if se.Msg != "Esta conexão não é de arquivos." {
		t.Errorf("mensagem: %q", se.Msg)
	}
	if !s.Running() {
		t.Error("o Reload recusado derrubou o toolbox")
	}
}

// escreveDbf grava um .dbf mínimo (dBase III, um campo NOME de 20 caracteres, só ASCII).
func escreveDbf(t *testing.T, caminho string, nomes ...string) {
	t.Helper()
	const nomeL = 20
	h := make([]byte, 32)
	h[0] = 0x03
	binary.LittleEndian.PutUint32(h[4:], uint32(len(nomes)))
	binary.LittleEndian.PutUint16(h[8:], 32+32+1)
	binary.LittleEndian.PutUint16(h[10:], 1+nomeL)
	campo := make([]byte, 32)
	copy(campo, "NOME")
	campo[11] = 'C'
	campo[16] = nomeL
	b := append(h, campo...)
	b = append(b, 0x0D)
	for _, n := range nomes {
		reg := []byte(strings.Repeat(" ", 1+nomeL))
		copy(reg[1:], n)
		b = append(b, reg...)
	}
	b = append(b, 0x1A)
	deve(t, os.WriteFile(caminho, b, 0o644))
}

func TestStartEReloadDbf(t *testing.T) {
	s := newSup(t, 10*time.Second)
	pasta := t.TempDir()
	escreveDbf(t, filepath.Join(pasta, "CLIENTE.DBF"), "Padaria do Ze", "Mercado Bom")
	conf := dbconf.Config{Engine: "dbf", Name: pasta}
	if err := s.Start(conf); err != nil {
		t.Fatalf("Start: %v", err)
	}
	destino := filepath.Join(s.opt.CacheDir, "dbf.sqlite")
	if _, err := os.Stat(destino); err != nil {
		t.Fatalf("dbf.sqlite não existe: %v", err)
	}
	if _, err := os.Stat(destino + ".novo"); !os.IsNotExist(err) {
		t.Errorf("sobrou dbf.sqlite.novo (err %v)", err)
	}
	if s.Planilhas() != nil {
		t.Errorf("DBF não tem relatório de planilhas: %+v", s.Planilhas())
	}
	escreveDbf(t, filepath.Join(pasta, "PRODUTO.DBF"), "Arroz")
	if err := s.Reload(conf); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if !s.Running() {
		t.Error("depois do Reload o toolbox deveria estar de pé")
	}
	if n := contaTabelas(t, destino); n != 2 {
		t.Errorf("depois do Reload: %d tabelas, quero 2", n)
	}
	if s.Planilhas() != nil {
		t.Errorf("DBF não tem relatório de planilhas: %+v", s.Planilhas())
	}
}
